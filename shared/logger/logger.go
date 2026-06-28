// Package logger padroniza o slog em JSON para todos os serviços.
// Ver ADR-0006 (observabilidade) — cada serviço nasce instrumentado.
package logger

import (
	"log/slog"
	"os"
)

// New cria um logger JSON com o nome do serviço anexado a todos os registros.
func New(service string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(h).With(slog.String("service", service))
}
