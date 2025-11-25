package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	StravaClientID     string
	StravaClientSecret string
	StravaCallbackURL  string
	SessionSecret      string
	Port               string
}

func Load() (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	cfg := &Config{
		StravaClientID:     os.Getenv("STRAVA_CLIENT_ID"),
		StravaClientSecret: os.Getenv("STRAVA_CLIENT_SECRET"),
		StravaCallbackURL:  getEnv("STRAVA_CALLBACK_URL", "http://localhost:8080/auth/callback"),
		SessionSecret:      getEnv("SESSION_SECRET", "super-secret-key"),
		Port:               getEnv("PORT", "8080"),
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
