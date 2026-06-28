// Package config lê configuração a partir de variáveis de ambiente.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config agrupa as configurações comuns de um serviço.
type Config struct {
	Port          string
	DatabaseURL   string // DSN do Postgres
	JWTPrivateKey string // chave Ed25519 em PEM; vazio em dev -> gera efêmera

	// Tracing (ADR-0014). OTLPEndpoint vazio desliga o tracing — dev local sem
	// Tempo no ar roda normal; o compose seta a env para apontar ao coletor.
	OTLPEndpoint     string  // OTLP gRPC host:port; via OTEL_EXPORTER_OTLP_ENDPOINT
	TraceSampleRatio float64 // fração de traces-raiz amostrados (ParentBased)
}

// Load lê o ambiente, com defaults convenientes para dev local.
func Load() Config {
	return Config{
		Port:             env("PORT", "8080"),
		DatabaseURL:      env("DATABASE_URL", defaultDSN()),
		JWTPrivateKey:    os.Getenv("JWT_PRIVATE_KEY"),
		OTLPEndpoint:     os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		TraceSampleRatio: envFloat("OTEL_TRACES_SAMPLER_RATIO", 1.0),
	}
}

func envFloat(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func defaultDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		env("DB_USER", "dev"), env("DB_PASSWORD", "dev"),
		env("DB_HOST", "localhost"), env("DB_PORT", "5432"),
		env("DB_NAME", "gamebackend"))
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}
