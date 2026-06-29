package domain

import "errors"

// ErrNoServer indica que o session-service não tem game server livre para a
// sessão (mapeado de codes.ResourceExhausted pelo adapter do session).
var ErrNoServer = errors.New("matchmaking: no server available")
