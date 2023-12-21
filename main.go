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

	"github.com/joho/godotenv"
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

type UnpublishedClubUser struct {
	Email    string  `json:"email"`
	Uid      *string `json:"uid"`
	ClubName string  `json:"clubName"`
}

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	redisAddress := os.Getenv("REDIS_ADDRESS")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisUsername := os.Getenv("REDIS_USERNAME")

	// FSA = Firebase Service Account
	fsaProjectId := os.Getenv("FSA_PROJECT_ID")
	fsaPrivateKey := os.Getenv("FSA_PRIVATE_KEY")
	fsaPrivateKeyId := os.Getenv("FSA_PRIVATE_KEY_ID")
	fsaClientEmail := os.Getenv("FSA_CLIENT_EMAIL")

	/**
	 * Check if REDIS_URL and REDIS_PASSWORD are set
	 */
	if redisAddress == "" || redisPassword == "" || redisUsername == "" || fsaPrivateKey == "" || fsaPrivateKeyId == "" || fsaProjectId == "" || fsaClientEmail == "" {
		fmt.Println("Please check correct environment variables are set")
		os.Exit(1)
	}

	redisUrl, err := url.Parse(redisAddress)
	if err != nil {
		log.Fatal(err)
	}

	/**
	 * Intialize redis client
	 */
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisUrl.Host,
		Password: redisPassword, // no password set
		Username: redisUsername,
		DB:       0, // use default DB
	})

	// needed to replace newlines in private key
	fsaPrivateKey = strings.ReplaceAll(fsaPrivateKey, "\n", "\\n")

	/**
	 * Initialize Firebase Admin SDK
	 */
	opt := option.WithCredentialsJSON([]byte(`{
		"type": "service_account",
		"private_key": "` + fsaPrivateKey + `",
		"private_key_id": "` + fsaPrivateKeyId + `",
		"project_id": "` + fsaProjectId + `",
		"client_email": "` + fsaClientEmail + `"
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

	unpublishedClubUsers := []UnpublishedClubUser{}

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
						unpublishedClubUsers = append(unpublishedClubUsers, UnpublishedClubUser{
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
	 * Now let's fetch the email based on uid
	 */
	for i := range unpublishedClubUsers {

		// get user by uid
		userRecord, err := client.GetUser(ctx, *unpublishedClubUsers[i].Uid)
		if err != nil {
			fmt.Printf("Error fetching user data: %v\n", err)
			continue
		}

		// get user email
		unpublishedClubUsers[i].Email = userRecord.Email
	}

	/**
	 * Turn unpublishedClubUsers into JSON, and write to file
	 */
	file, _ := json.MarshalIndent(unpublishedClubUsers, "", " ")
	_ = os.WriteFile("unpublished-club-users.json", file, 0644)

}
