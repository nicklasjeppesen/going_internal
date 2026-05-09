package util

import (
	"os"
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
