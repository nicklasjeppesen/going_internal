package util

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func GetEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func GetURL() string {
	var host = os.Getenv("APP_URL")
	var port = os.Getenv("APP_PORT")
	return host + ":" + port
}

func LoadEnv() {
	enverr := godotenv.Load()
	if enverr != nil {
		log.Fatalf("Error loading .env file")
	}
}
