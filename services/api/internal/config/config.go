package config

import (
	"os"
)

type Config struct {
	Env           string
	Port          string
	DatabaseURL   string
	RedisURL      string
	EncryptionKey string

	// Aggregator credentials
	Akoya    AkoyaConfig
	Finicity FinicityConfig

	// Enrichment service
	EnrichmentURL string
}

type AkoyaConfig struct {
	ClientID     string
	ClientSecret string
	BaseURL      string // IDP base — auth/token endpoints
	DataURL      string // Data API base — accounts/transactions endpoints
}

type FinicityConfig struct {
	PartnerID     string
	PartnerSecret string
	AppKey        string
	BaseURL       string
}

func Load() *Config {
	return &Config{
		Env:           getEnv("ENV", "development"),
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://hound:hound@localhost:5432/hound?sslmode=disable"),
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379"),
		EncryptionKey: mustGetEnv("ENCRYPTION_KEY"),
		EnrichmentURL: getEnv("ENRICHMENT_URL", "http://localhost:8081"),

		Akoya: AkoyaConfig{
			ClientID:     getEnv("AKOYA_CLIENT_ID", ""),
			ClientSecret: getEnv("AKOYA_CLIENT_SECRET", ""),
			BaseURL:      getEnv("AKOYA_BASE_URL", "https://sandbox-idp.ddp.akoya.com"),
			DataURL:      getEnv("AKOYA_DATA_URL", "https://sandbox-products.ddp.akoya.com"),
		},

		Finicity: FinicityConfig{
			PartnerID:     getEnv("FINICITY_PARTNER_ID", ""),
			PartnerSecret: getEnv("FINICITY_PARTNER_SECRET", ""),
			AppKey:        getEnv("FINICITY_APP_KEY", ""),
			BaseURL:       getEnv("FINICITY_BASE_URL", "https://api.finicity.com"),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (c *Config) BaseURL() string {
	return getEnv("BASE_URL", "http://localhost:8080")
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" && os.Getenv("ENV") == "production" {
		panic("required env var not set: " + key)
	}
	// Allow empty in development
	return v
}
