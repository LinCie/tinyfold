package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	Port             int
	DatabaseURL      string
	JWTAccessSecret  string
	JWTRefreshSecret string
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

	accessSecret := os.Getenv("JWT_ACCESS_SECRET")
	if len(accessSecret) < 32 {
		return nil, errors.New("JWT_ACCESS_SECRET must be at least 32 characters")
	}
	refreshSecret := os.Getenv("JWT_REFRESH_SECRET")
	if len(refreshSecret) < 32 {
		return nil, errors.New("JWT_REFRESH_SECRET must be at least 32 characters")
	}

	return &Config{
		DatabaseURL:      databaseURL,
		Port:             port,
		JWTAccessSecret:  accessSecret,
		JWTRefreshSecret: refreshSecret,
	}, nil
}
