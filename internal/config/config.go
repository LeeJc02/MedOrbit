package config

import "os"

type Config struct {
	HTTPAddr     string
	GRPCAddr     string
	JWTSecret    string
	PostgresDSN  string
	AuditEnabled bool
}

func Load() Config {
	return Config{
		HTTPAddr:     getenv("DDI_HTTP_ADDR", ":8080"),
		GRPCAddr:     getenv("DDI_GRPC_ADDR", "127.0.0.1:50051"),
		JWTSecret:    getenv("DDI_JWT_SECRET", "dev-secret"),
		PostgresDSN:  getenv("DDI_PG_DSN", "postgres://ddi:ddi@localhost:5432/ddi?sslmode=disable"),
		AuditEnabled: getenv("DDI_AUDIT_ENABLED", "true") == "true",
	}
}

func getenv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}
