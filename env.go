package main

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type EnvVars struct {
	RedisAddress    string
	RedisPassword   string
	RedisUsername   string
	FSAProjectId    string
	FSAPrivateKey   string
	FSAPrivateKeyId string
	FSAClientEmail  string
}

func LoadEnvVars() (*EnvVars, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	redisAddress := os.Getenv("REDIS_ADDRESS")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisUsername := os.Getenv("REDIS_USERNAME")

	// FSA = Firebase Service Account
	fsaProjectId := os.Getenv("FSA_PROJECT_ID")
	fsaPrivateKey := os.Getenv("FSA_PRIVATE_KEY")
	fsaPrivateKeyId := os.Getenv("FSA_PRIVATE_KEY_ID")
	fsaClientEmail := os.Getenv("FSA_CLIENT_EMAIL")

	if redisAddress == "" || redisPassword == "" || redisUsername == "" || fsaPrivateKey == "" || fsaPrivateKeyId == "" || fsaProjectId == "" || fsaClientEmail == "" {
		return nil, errors.New("missing environment variables")
	}

	return &EnvVars{
		RedisAddress:    redisAddress,
		RedisPassword:   redisPassword,
		RedisUsername:   redisUsername,
		FSAProjectId:    fsaProjectId,
		FSAPrivateKey:   fsaPrivateKey,
		FSAPrivateKeyId: fsaPrivateKeyId,
		FSAClientEmail:  fsaClientEmail,
	}, nil

}
