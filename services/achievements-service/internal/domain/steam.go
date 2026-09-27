package domain

import (
	"context"
	"errors"
	"time"
)

// Espelhamento de conquistas na Steam (spec 019). O achievements continua a
// fonte da verdade; a Steam só recebe o desbloqueio, de forma assíncrona.

// Estados do espelhamento de um desbloqueio (player_achievements.steam_sync_state).
const (
	SteamSyncPending = ""        // ainda não espelhado (ou reagendado)
	SteamSyncSynced  = "synced"  // a Steam confirmou
	SteamSyncSkipped = "skipped" // jogador sem conta Steam vinculada: não tenta de novo
	SteamSyncFailed  = "failed"  // desistiu após o limite de tentativas
)

// ErrNoSteamAccount indica que o jogador não tem conta Steam vinculada.
var ErrNoSteamAccount = errors.New("achievements: player has no linked steam account")

// SteamSyncItem é um desbloqueio a espelhar.
type SteamSyncItem struct {
	GameID        string
	PlayerID      string
	AchievementID string
	SteamAPIName  string
	Attempts      int // tentativas já feitas antes desta
}

// SteamSyncRepository é a fila de desbloqueios a espelhar (Port de saída).
type SteamSyncRepository interface {
	// ClaimSteamPending pega até limit desbloqueios pendentes do jogo cuja vez
	// chegou (next_attempt vencido) e os "aluga" até leaseUntil — outra réplica
	// não pega o mesmo item nesse intervalo.
	ClaimSteamPending(ctx context.Context, gameID string, limit int, now, leaseUntil time.Time) ([]SteamSyncItem, error)
	// MarkSteamDone grava o estado final (synced/skipped/failed) com o motivo.
	MarkSteamDone(ctx context.Context, item SteamSyncItem, state, reason string, at time.Time) error
	// MarkSteamRetry conta a tentativa e reagenda para nextAt com o erro.
	MarkSteamRetry(ctx context.Context, item SteamSyncItem, nextAt time.Time, reason string) error
}

// SteamIDResolver devolve o SteamID64 do jogador (vínculo do identity).
// ErrNoSteamAccount se ele não tiver conta Steam.
type SteamIDResolver interface {
	SteamID(ctx context.Context, playerID string) (string, error)
}

// SteamAchievementSetter marca a conquista na Steam do jogador.
type SteamAchievementSetter interface {
	SetAchievement(ctx context.Context, steamID, apiName string) error
}
