package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"

	"google.golang.org/api/option"
	"gopkg.in/yaml.v2"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

type ClubConfig struct {
	Name            string        `json:"name"`
	TwitterHandle   string        `json:"twitterHandle"`
	Description     string        `json:"description"`
	Url             string        `json:"url"`
	PropertyAddress string        `json:"propertyAddress"`
	AdminRolePoints int           `json:"adminRolePoints"`
	ChainId         int           `json:"chainId"`
	RpcUrl          string        `json:"rpcUrl"`
	Options         *[]ClubOption `json:"options"`
	Plugins         *[]Plugin     `json:"plugins"`
}

type Plugin struct{}

type ClubOption struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type ClubUser struct {
	Email    string  `json:"email"`
	Uid      *string `json:"uid"`
	ClubName string  `json:"clubName"`
}

func main() {

	env, err := LoadEnvVars()
	if err != nil {
		log.Fatal("Error loading config: ", err)
	}

	redisUrl, err := url.Parse(env.RedisAddress)
	if err != nil {
		log.Fatal(err)
	}

	/**
	 * Intialize redis client
	 */
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisUrl.Host,
		Password: env.RedisPassword, // no password set
		Username: env.RedisUsername,
		DB:       0, // use default DB
	})

	// needed to replace newlines in private key
	fsaPrivateKey := strings.ReplaceAll(env.FSAPrivateKey, "\n", "\\n")

	/**
	 * Initialize Firebase Admin SDK
	 */
	opt := option.WithCredentialsJSON([]byte(`{
		"type": "service_account",
		"private_key": "` + fsaPrivateKey + `",
		"private_key_id": "` + env.FSAPrivateKeyId + `",
		"project_id": "` + env.FSAProjectId + `",
		"client_email": "` + env.FSAClientEmail + `"
	}`))

	firebase, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		panic(err)
	}

	client, err := firebase.Auth(ctx)
	if err != nil {
		panic(err)
	}

	// fetch all keys from redis
	keys, err := rdb.Keys(ctx, "*").Result()
	if err != nil {
		panic(err)
	}

	unpublishedClubUsers := []ClubUser{}
	publishedClubUsers := []ClubUser{}

	for _, key := range keys {

		/*
		 * check if key string includes ":"
		 * if so, they are not a club, skip
		 */
		if strings.Contains(key, ":") {
			continue
		}

		/**
		 * Fetch the club by key
		 */
		encodedConfig, err := rdb.Get(ctx, key).Result()
		if err != nil {
			panic(err)
		}

		/**
		 * Decode the club config to YAML string
		 */
		decodedConfig, err := base64.StdEncoding.DecodeString(encodedConfig)
		if err != nil {
			panic(err)
		}

		/**
		 * Convert YAML string to ClubConfig struct
		 */
		var clubConfig ClubConfig

		err = yaml.Unmarshal(decodedConfig, &clubConfig)
		if err != nil {
			fmt.Printf("Error parsing YAML for: %s\n", string(decodedConfig)) // Add this line
			fmt.Printf("Error parsing YAML: %v\n", err)                       // Add this line
			fmt.Println("-------------------")
			// skip to next club
			continue
		}

		/**
		 * Find draft options
		 */

		if clubConfig.Options != nil {
			for _, option := range *clubConfig.Options {
				if option.Key == "__draft" {

					valueMap, ok := option.Value.(map[interface{}]interface{})
					if !ok {
						log.Fatal("Value is not a map")
					}

					isInDraft, ok := valueMap["isInDraft"].(bool)
					if !ok {
						log.Printf("%s is not a bool for isInDraft", clubConfig.Name)
						continue
					}

					uid, ok := valueMap["uid"].(string)
					if !ok {
						continue
					}

					/**
					 * If isInDraft is true, and uid is not empty string
					 * add to unpublishedClubUsers
					 */
					if isInDraft && uid != "" {
						unpublishedClubUsers = append(unpublishedClubUsers, ClubUser{
							Email:    "",
							Uid:      &uid,
							ClubName: clubConfig.Name,
						})
					}

					/**
					 * If isInDraft is false, and uid is not empty string
					 * add to publishedClubUsers
					 */
					if !isInDraft && uid != "" {
						publishedClubUsers = append(publishedClubUsers, ClubUser{
							Email:    "",
							Uid:      &uid,
							ClubName: clubConfig.Name,
						})
					}
				}
			}
		}
	} // end loop through keys

	/**
	 * Now let's fetch the email of unpublished club users based on uid
	 */
	for i := range unpublishedClubUsers {

		// get user by uid
		email, err := fetchUserEmailByUid(*unpublishedClubUsers[i].Uid, client)
		if err != nil {
			log.Printf("Error fetching user email for uid: %s", *unpublishedClubUsers[i].Uid)
			continue
		}

		// get user email
		unpublishedClubUsers[i].Email = email
	}

	/**
	 * Now let's fetch the email of unpublished club users based on uid
	 */
	for i := range publishedClubUsers {

		// get user by uid
		email, err := fetchUserEmailByUid(*publishedClubUsers[i].Uid, client)
		if err != nil {
			log.Printf("Error fetching user email for uid: %s", *publishedClubUsers[i].Uid)
			continue
		}

		// get user email
		publishedClubUsers[i].Email = email
	}

	/**
	 * Turn unpublishedClubUsers into JSON, and write to file
	 */
	unpublishedFile, _ := json.MarshalIndent(unpublishedClubUsers, "", " ")
	_ = os.WriteFile("club-users-unpublished.json", unpublishedFile, 0644)

	/**
	 * Turn unpublishedClubUsers into JSON, and write to file
	 */
	publishedFile, _ := json.MarshalIndent(publishedClubUsers, "", " ")
	_ = os.WriteFile("club-users-published.json", publishedFile, 0644)
}

func fetchUserEmailByUid(uid string, client *auth.Client) (string, error) {
	userRecord, err := client.GetUser(ctx, uid)
	if err != nil {
		return "", err
	}

	return userRecord.Email, nil
}
