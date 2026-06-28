package domain

import "errors"

// ErrNotFound indica que o jogador não tem save. O transport o mapeia para 404.
// A autorização é tratada pelo shared/auth no middleware (401 lá).
var ErrNotFound = errors.New("cloud-save: save não encontrado")
