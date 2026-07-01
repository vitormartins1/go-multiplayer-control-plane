// Package storage implementa a Port LeaderboardStore em Redis (sorted set)
// via go-redis/v9 (ADR-0015). Ver ADR-0002.
package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/vitormartins1/go-game-backend/services/leaderboard-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos a Port.
var _ domain.LeaderboardStore = (*RedisStore)(nil)

// RedisStore guarda um sorted set por jogo: a key é prefix + ":" + gameID.
type RedisStore struct {
	client *redis.Client
	prefix string
}

// NewRedisStore cria o store a partir do endereço do Redis (host:port). O prefix
// é o namespace das keys; cada jogo vira "<prefix>:<gameID>".
func NewRedisStore(addr, prefix string) *RedisStore {
	return &RedisStore{
		client: redis.NewClient(&redis.Options{Addr: addr}),
		prefix: prefix,
	}
}

// key monta a key do sorted set do jogo.
func (s *RedisStore) key(gameID string) string {
	return s.prefix + ":" + gameID
}

// Ping verifica a conectividade (usado no boot, com retry).
func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// Submit aplica highscore: ZADD GT só sobe o score se o novo for maior.
func (s *RedisStore) Submit(ctx context.Context, gameID, playerID string, score int64) error {
	err := s.client.ZAddArgs(ctx, s.key(gameID), redis.ZAddArgs{
		GT:      true,
		Members: []redis.Z{{Score: float64(score), Member: playerID}},
	}).Err()
	if err != nil {
		return fmt.Errorf("leaderboard storage: submit: %w", err)
	}
	return nil
}

// Rank devolve a posição (0-based) e o score do jogador; ErrNotRanked se ausente.
func (s *RedisStore) Rank(ctx context.Context, gameID, playerID string) (int64, int64, error) {
	res, err := s.client.ZRevRankWithScore(ctx, s.key(gameID), playerID).Result()
	if errors.Is(err, redis.Nil) {
		return 0, 0, domain.ErrNotRanked
	}
	if err != nil {
		return 0, 0, fmt.Errorf("leaderboard storage: rank: %w", err)
	}
	return res.Rank, int64(res.Score), nil
}

func (s *RedisStore) Top(ctx context.Context, gameID string, limit int) ([]domain.Entry, error) {
	return s.Range(ctx, gameID, 0, int64(limit-1))
}

func (s *RedisStore) Range(ctx context.Context, gameID string, start, stop int64) ([]domain.Entry, error) {
	zs, err := s.client.ZRevRangeWithScores(ctx, s.key(gameID), start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("leaderboard storage: range: %w", err)
	}
	entries := make([]domain.Entry, 0, len(zs))
	for i, z := range zs {
		member, _ := z.Member.(string)
		entries = append(entries, domain.Entry{
			Rank:     start + int64(i) + 1, // posição (0-based) -> rank 1-based
			PlayerID: member,
			Score:    int64(z.Score),
		})
	}
	return entries, nil
}
