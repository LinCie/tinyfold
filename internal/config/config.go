package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	Port        int
	DatabaseURL string
}

func NewConfig() (*Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is not defined")
	}

	port := 8080
	if v := os.Getenv("PORT"); v != "" {
		parsedPort, err := strconv.Atoi(v)
		if err != nil {
			return &Config{}, err
		}

		port = parsedPort
	}

	return &Config{
		DatabaseURL: databaseURL,
		Port:        port,
	}, nil
}
