package domain

import "errors"

// Erros de domínio do player-data-service. O transport mapeia cada um para um
// HTTP status code (ver ADR-0002). A autorização é tratada pelo shared/auth no
// middleware (mapeada a 401 lá).
var (
	ErrInvalidKey   = errors.New("player-data: namespace ou key inválido")
	ErrInvalidValue = errors.New("player-data: value não é JSON válido")
	ErrNotFound     = errors.New("player-data: não encontrado")
)
