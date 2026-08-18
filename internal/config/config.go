package config

import "os"

type Config struct {
	DatabaseUrl string
	RedisAddr string 
	HTTPAddr string
}


func Load() Config {
	return Config{
		DatabaseUrl: getenv("DATABASE_URL" , "postgres://ledger:ledger@postgres:5432/ledger?sslmode=disable"),
		RedisAddr: getenv("REDIS_ADDR", "redis:6379"),
		HTTPAddr: getenv("HTTP_ADDR", ":8080"),
	}
}

// return env var else default value from above struct
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}