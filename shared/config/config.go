// Package config lê configuração a partir de variáveis de ambiente.
package config

import (
	"fmt"
	"os"
)

// Config agrupa as configurações comuns de um serviço.
type Config struct {
	Port          string
	DatabaseURL   string // DSN do Postgres
	JWTPrivateKey string // chave Ed25519 em PEM; vazio em dev -> gera efêmera
}

// Load lê o ambiente, com defaults convenientes para dev local.
func Load() Config {
	return Config{
		Port:          env("PORT", "8080"),
		DatabaseURL:   env("DATABASE_URL", defaultDSN()),
		JWTPrivateKey: os.Getenv("JWT_PRIVATE_KEY"),
	}
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
