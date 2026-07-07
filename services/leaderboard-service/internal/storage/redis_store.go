// Package storage implementa a Port LeaderboardStore em Redis (sorted set)
// via go-redis/v9 (ADR-0015). Ver ADR-0002.
package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/leaderboard-service/internal/domain"
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

// key monta a key do sorted set do (jogo, board).
func (s *RedisStore) key(gameID, board string) string {
	return s.prefix + ":" + gameID + ":" + board
}

// processedKey é o SET de matchIDs já aplicados nos boards cumulativos do jogo
// (guarda de idempotência do ZINCRBY, ADR-0023 A2).
func (s *RedisStore) processedKey(gameID string) string {
	return s.prefix + ":processed:" + gameID
}

// applyCumulativeScript aplica os ZINCRBY de uma partida sob uma guarda de
// idempotência, atômico (o Redis executa o script inteiro sem interleaving).
// KEYS[1] = SET processed; ARGV[1] = matchID; ARGV[2..] = trios (boardKey,
// member, delta). Se o matchID já estava no SET (SADD devolve 0) -> no-op e
// retorna 1 (dup); senão soma cada parcela e retorna 0. É o análogo Redis da tx
// idempotente do achievements (Postgres): SADD+ZINCRBY num só passo indivisível.
var applyCumulativeScript = redis.NewScript(`
if redis.call('SADD', KEYS[1], ARGV[1]) == 0 then
  return 1
end
for i = 2, #ARGV, 3 do
  redis.call('ZINCRBY', ARGV[i], ARGV[i+2], ARGV[i+1])
end
return 0
`)

// ApplyCumulative soma as parcelas nos boards cumulativos de uma partida (ZINCRBY)
// idempotente por matchID via applyCumulativeScript. Sem parcelas -> no-op.
func (s *RedisStore) ApplyCumulative(ctx context.Context, gameID, matchID string, incrs []domain.Increment) (bool, error) {
	if len(incrs) == 0 {
		return false, nil // nada a somar: não há o que deduplicar
	}
	argv := make([]any, 0, 1+len(incrs)*3)
	argv = append(argv, matchID)
	for _, inc := range incrs {
		argv = append(argv, s.key(gameID, inc.Board), inc.PlayerID, inc.Delta)
	}
	res, err := applyCumulativeScript.Run(ctx, s.client, []string{s.processedKey(gameID)}, argv...).Int()
	if err != nil {
		return false, fmt.Errorf("leaderboard storage: apply cumulative: %w", err)
	}
	return res == 1, nil // 1 = partida já aplicada (dup)
}

// Ping verifica a conectividade (usado no boot, com retry).
func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// Submit aplica highscore: ZADD GT só sobe o score se o novo for maior.
func (s *RedisStore) Submit(ctx context.Context, gameID, board, playerID string, score int64) error {
	err := s.client.ZAddArgs(ctx, s.key(gameID, board), redis.ZAddArgs{
		GT:      true,
		Members: []redis.Z{{Score: float64(score), Member: playerID}},
	}).Err()
	if err != nil {
		return fmt.Errorf("leaderboard storage: submit: %w", err)
	}
	return nil
}

// ForceScore substitui o score independente do valor atual (admin override, ZADD sem GT).
func (s *RedisStore) ForceScore(ctx context.Context, gameID, board, playerID string, score int64) error {
	err := s.client.ZAdd(ctx, s.key(gameID, board), redis.Z{Score: float64(score), Member: playerID}).Err()
	if err != nil {
		return fmt.Errorf("leaderboard storage: force score: %w", err)
	}
	return nil
}

// Rank devolve a posição (0-based) e o score do jogador; ErrNotRanked se ausente.
func (s *RedisStore) Rank(ctx context.Context, gameID, board, playerID string) (int64, int64, error) {
	res, err := s.client.ZRevRankWithScore(ctx, s.key(gameID, board), playerID).Result()
	if errors.Is(err, redis.Nil) {
		return 0, 0, domain.ErrNotRanked
	}
	if err != nil {
		return 0, 0, fmt.Errorf("leaderboard storage: rank: %w", err)
	}
	return res.Rank, int64(res.Score), nil
}

func (s *RedisStore) Top(ctx context.Context, gameID, board string, limit int) ([]domain.Entry, error) {
	return s.Range(ctx, gameID, board, 0, int64(limit-1))
}

func (s *RedisStore) Range(ctx context.Context, gameID, board string, start, stop int64) ([]domain.Entry, error) {
	zs, err := s.client.ZRevRangeWithScores(ctx, s.key(gameID, board), start, stop).Result()
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
