// Package storage implementa a Port LeaderboardStore em Redis (sorted set)
// via go-redis/v9 (ADR-0015). Ver ADR-0002.
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/leaderboard-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos a Port.
var _ domain.LeaderboardStore = (*RedisStore)(nil)

// TTL das keys periódicas (spec 007): só limpeza de memória — a correção vem
// do período no nome da key, então renovar a cada write (sliding) é inofensivo.
// Retenção ≥ 1 período anterior deixa a porta aberta para consultas passadas.
const (
	dailyTTL   = 3 * 24 * time.Hour
	weeklyTTL  = 15 * 24 * time.Hour
	monthlyTTL = 45 * 24 * time.Hour
)

// RedisStore guarda um sorted set por (jogo, board, janela): a key all-time é
// prefix + ":" + gameID + ":" + board (pré-janelas, intacta); as periódicas
// ganham o período no sufixo (spec 007).
type RedisStore struct {
	client *redis.Client
	prefix string
	// now permite fixar o relógio nos testes (períodos determinísticos, virada
	// de janela). Produção: time.Now.
	now func() time.Time
}

// NewRedisStore cria o store a partir do endereço do Redis (host:port). O prefix
// é o namespace das keys; cada jogo vira "<prefix>:<gameID>".
func NewRedisStore(addr, prefix string) *RedisStore {
	return &RedisStore{
		client: redis.NewClient(&redis.Options{Addr: addr}),
		prefix: prefix,
		now:    time.Now,
	}
}

// key monta a key do sorted set all-time do (jogo, board) — a key pré-janelas.
func (s *RedisStore) key(gameID, board string) string {
	return s.prefix + ":" + gameID + ":" + board
}

// windowKey monta a key do sorted set do (jogo, board, janela) para o instante
// t (UTC): alltime = key existente; daily/weekly/monthly ganham o período no
// sufixo (weekly usa a semana ISO). window deve ser válida (service valida).
func (s *RedisStore) windowKey(gameID, board, window string, t time.Time) string {
	base := s.key(gameID, board)
	t = t.UTC()
	switch window {
	case domain.WindowDaily:
		return base + ":d:" + t.Format("20060102")
	case domain.WindowWeekly:
		y, w := t.ISOWeek()
		return fmt.Sprintf("%s:w:%04dW%02d", base, y, w)
	case domain.WindowMonthly:
		return base + ":m:" + t.Format("200601")
	default: // domain.WindowAllTime
		return base
	}
}

// windowTTL é o TTL da key periódica da janela; 0 = sem TTL (all-time).
func windowTTL(window string) time.Duration {
	switch window {
	case domain.WindowDaily:
		return dailyTTL
	case domain.WindowWeekly:
		return weeklyTTL
	case domain.WindowMonthly:
		return monthlyTTL
	default:
		return 0
	}
}

// processedTTL é a janela de idempotência dos boards cumulativos (spec 008):
// cobre a retenção máxima da fila (14 dias na AWS) + a da DLQ (14) com folga.
const processedTTL = 30 * 24 * time.Hour

// processedKey marca UMA partida já aplicada nos boards cumulativos do jogo
// (guarda de idempotência do ZINCRBY, ADR-0023 A2), com TTL de processedTTL
// (spec 008). Única para todas as janelas: reprocessar é no-op em todas.
func (s *RedisStore) processedKey(gameID, matchID string) string {
	return s.prefix + ":processed:" + gameID + ":" + matchID
}

// legacyProcessedKey é o SET único de matchIDs usado até a spec 008. Só é
// consultado (nunca mais escrito) para uma reentrega de partida aplicada
// antes do deploy continuar detectada; pode ser apagado 30 dias depois.
func (s *RedisStore) legacyProcessedKey(gameID string) string {
	return s.prefix + ":processed:" + gameID
}

// applyCumulativeScript aplica os ZINCRBY de uma partida sob uma guarda de
// idempotência, atômico (o Redis executa o script inteiro sem interleaving).
// KEYS[1] = key da partida; KEYS[2] = SET legado; ARGV[1] = matchID;
// ARGV[2] = TTL da guarda em segundos; ARGV[3..] = quádruplas (boardKey,
// member, delta, ttlSeconds — 0 = sem EXPIRE). Se a partida já foi aplicada
// (key presente ou matchID no SET legado) -> no-op e retorna 1 (dup); senão
// grava a key com TTL, soma cada parcela (e renova o TTL das keys periódicas)
// e retorna 0. É o análogo Redis da tx idempotente do achievements
// (Postgres): guarda + ZINCRBY num só passo indivisível.
var applyCumulativeScript = redis.NewScript(`
if redis.call('SISMEMBER', KEYS[2], ARGV[1]) == 1 then
  return 1
end
if not redis.call('SET', KEYS[1], 1, 'NX', 'EX', ARGV[2]) then
  return 1
end
for i = 3, #ARGV, 4 do
  redis.call('ZINCRBY', ARGV[i], ARGV[i+2], ARGV[i+1])
  local ttl = tonumber(ARGV[i+3])
  if ttl > 0 then
    redis.call('EXPIRE', ARGV[i], ttl)
  end
end
return 0
`)

// ApplyCumulative soma as parcelas nos boards cumulativos de uma partida
// (ZINCRBY em TODAS as janelas, spec 007) idempotente por matchID via
// applyCumulativeScript. Sem parcelas -> no-op.
func (s *RedisStore) ApplyCumulative(ctx context.Context, gameID, matchID string, incrs []domain.Increment) (bool, error) {
	if len(incrs) == 0 {
		return false, nil // nada a somar: não há o que deduplicar
	}
	t := s.now()
	argv := make([]any, 0, 2+len(incrs)*len(domain.Windows)*4)
	argv = append(argv, matchID, int64(processedTTL.Seconds()))
	for _, inc := range incrs {
		for _, w := range domain.Windows {
			argv = append(argv, s.windowKey(gameID, inc.Board, w, t), inc.PlayerID, inc.Delta,
				int64(windowTTL(w).Seconds()))
		}
	}
	res, err := applyCumulativeScript.Run(ctx, s.client, []string{s.processedKey(gameID, matchID), s.legacyProcessedKey(gameID)}, argv...).Int()
	if err != nil {
		return false, fmt.Errorf("leaderboard storage: apply cumulative: %w", err)
	}
	return res == 1, nil // 1 = partida já aplicada (dup)
}

// Ping verifica a conectividade (usado no boot, com retry).
func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// spreadScore aplica o score nas 4 janelas via pipeline (gt escolhe ZADD GT ou
// ZADD puro) e renova o TTL das periódicas. Pipeline não é atômico entre keys,
// mas cada ZADD é idempotente/convergente — retry é seguro.
func (s *RedisStore) spreadScore(ctx context.Context, gameID, board, playerID string, score int64, gt bool) error {
	t := s.now()
	_, err := s.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, w := range domain.Windows {
			k := s.windowKey(gameID, board, w, t)
			pipe.ZAddArgs(ctx, k, redis.ZAddArgs{
				GT:      gt,
				Members: []redis.Z{{Score: float64(score), Member: playerID}},
			})
			if ttl := windowTTL(w); ttl > 0 {
				pipe.Expire(ctx, k, ttl)
			}
		}
		return nil
	})
	return err
}

// Submit aplica highscore: ZADD GT só sobe o score se o novo for maior.
func (s *RedisStore) Submit(ctx context.Context, gameID, board, playerID string, score int64) error {
	if err := s.spreadScore(ctx, gameID, board, playerID, score, true); err != nil {
		return fmt.Errorf("leaderboard storage: submit: %w", err)
	}
	return nil
}

// ForceScore substitui o score independente do valor atual (admin override, ZADD sem GT).
func (s *RedisStore) ForceScore(ctx context.Context, gameID, board, playerID string, score int64) error {
	if err := s.spreadScore(ctx, gameID, board, playerID, score, false); err != nil {
		return fmt.Errorf("leaderboard storage: force score: %w", err)
	}
	return nil
}

// Rank devolve a posição (0-based) e o score do jogador na janela corrente;
// ErrNotRanked se ausente.
func (s *RedisStore) Rank(ctx context.Context, gameID, board, window, playerID string) (int64, int64, error) {
	res, err := s.client.ZRevRankWithScore(ctx, s.windowKey(gameID, board, window, s.now()), playerID).Result()
	if errors.Is(err, redis.Nil) {
		return 0, 0, domain.ErrNotRanked
	}
	if err != nil {
		return 0, 0, fmt.Errorf("leaderboard storage: rank: %w", err)
	}
	return res.Rank, int64(res.Score), nil
}

func (s *RedisStore) Top(ctx context.Context, gameID, board, window string, limit int) ([]domain.Entry, error) {
	return s.Range(ctx, gameID, board, window, 0, int64(limit-1))
}

func (s *RedisStore) Range(ctx context.Context, gameID, board, window string, start, stop int64) ([]domain.Entry, error) {
	zs, err := s.client.ZRevRangeWithScores(ctx, s.windowKey(gameID, board, window, s.now()), start, stop).Result()
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
