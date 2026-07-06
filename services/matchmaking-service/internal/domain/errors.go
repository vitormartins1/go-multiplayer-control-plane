package domain

import "errors"

// ErrNoServer indica que o session-service não tem game server livre para a
// sessão (mapeado de codes.ResourceExhausted pelo adapter do session).
var ErrNoServer = errors.New("matchmaking: no server available")

// ErrProfileUnavailable indica que o tamanho do match não pôde ser derivado do
// perfil de multiplayer do jogo (ADR-0023): jogo sem perfil, arquétipo !=
// queue_arena, ou game-registry indisponível. O ticket não entra na fila.
var ErrProfileUnavailable = errors.New("matchmaking: multiplayer profile unavailable")
