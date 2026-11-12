package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	Port              string
	MongoURI          string
	MongoDatabase     string
	JWTSecret         string
	JWTDuration       time.Duration
	UserCountInterval time.Duration
	ShutdownTimeout   time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Port:              getEnv("PORT", "8080"),
		MongoURI:          getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:     getEnv("MONGO_DATABASE", "user_management"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		JWTDuration:       getDuration("JWT_DURATION", 24*time.Hour),
		UserCountInterval: getDuration("USER_COUNT_INTERVAL", 10*time.Second),
		ShutdownTimeout:   getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}

	// The API must not start with an empty or guessable default secret.
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must be set and at least 32 characters long")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}
