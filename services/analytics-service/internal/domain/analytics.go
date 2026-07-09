// Package domain contém os tipos puros e as Ports do analytics-service. Zero
// dependências externas (ADR-0002). Tudo escopado por gameID (ADR-0017).
package domain

import "context"

// Vocabulário do desfecho (ADR-0023, arquétipo queue_arena).
const (
	OutcomeWin  = "win"
	OutcomeLoss = "loss"
	OutcomeDraw = "draw"
)

// PlayerResult é o resultado de um jogador numa partida (do fan-out match-results).
// Outcome é o desfecho polimórfico (ADR-0023); Won permanece por compat/fallback.
type PlayerResult struct {
	PlayerID string
	Outcome  string // "win" | "loss" | "draw"
	Won      bool
	Score    int64
}

// MatchResult é o resultado de uma partida ingerido do fan-out (ADR-0020/0021).
// É a unidade de trabalho do worker pool.
type MatchResult struct {
	MatchID string
	GameID  string
	Results []PlayerResult
}

// BattleRoyaleTopNThreshold é o limiar do "top N" (política do sink, consistente
// com os demais sinks da BR3). No histograma do analytics os baldes são
// mutuamente exclusivos: o 1º lugar vai só para winners; top_n é 2..threshold.
const BattleRoyaleTopNThreshold = 10

// PlacementResult é o resultado por colocação (battle_royale, BR3) ingerido do
// fan-out (kind=placement, session/015). Unidade do caminho de placement do pool.
type PlacementResult struct {
	MatchID string
	GameID  string
	Entries []PlacementEntry
}

// PlacementEntry é a colocação de um participante. O analytics só agrega
// distribuição e sobrevivência (não guarda playerId).
type PlacementEntry struct {
	Placement  int
	SurvivedMs int64
}

// PlacementDistribution é o histograma de colocação de um jogo + sobrevivência
// média (read model, BR3). Aditivo ao GameAnalytics; nil fora do battle_royale.
type PlacementDistribution struct {
	Winners       int64 // colocação == 1
	TopN          int64 // 2..threshold
	Rest          int64 // > threshold
	Entries       int64 // participantes (winners+topN+rest)
	AvgSurvivedMs float64
}

// Vocabulário do desfecho coletivo (espelha o session, ADR-0023/C2).
const (
	MissionSuccess = "MISSION_SUCCESS"
	MissionFailure = "MISSION_FAILURE"
)

// CollectiveResult é o desfecho coletivo da missão (coop_mission, C3) ingerido do
// fan-out (kind=collective, session/016). O analytics só agrega o desfecho da
// SESSÃO (ignora players/stats). Unidade do caminho collective do pool.
type CollectiveResult struct {
	MatchID string
	GameID  string
	Outcome string // MISSION_SUCCESS | MISSION_FAILURE
}

// MissionDistribution é a distribuição de desfecho da missão de um jogo (read
// model, C3): sucessos/falhas + taxa de sucesso derivada. Aditivo ao
// GameAnalytics; nil fora do coop_mission.
type MissionDistribution struct {
	Successes   int64
	Failures    int64
	SuccessRate float64 // successes / (successes+failures); 0 se nenhuma missão
}

// GameStat é o rollup de um jogo (avgScore derivado no read).
type GameStat struct {
	GameID         string
	Matches        int64
	UniquePlayers  int64
	Participations int64
	Wins           int64
	Losses         int64
	Draws          int64
	AvgScore       float64
}

// Totals são os totais cross-game do overview.
type Totals struct {
	Matches       int64
	UniquePlayers int64 // distintos globais
	Games         int64 // jogos com atividade
}

// Overview é o panorama cross-game (aba Analytics): totais, jogos e a série de
// partidas por dia somada entre todos os jogos (spec 002).
type Overview struct {
	Totals Totals
	Games  []GameStat
	Daily  []DailyPoint
}

// DailyPoint é uma partida-por-dia da série de um jogo.
type DailyPoint struct {
	Day     string // YYYY-MM-DD (UTC)
	Matches int64
}

// GameAnalytics é o detalhe de um jogo (resumo + série diária + participações
// por papel, host_world_dropin — spec 004).
type GameAnalytics struct {
	GameStat
	Daily []DailyPoint
	// RoleParticipations é a quebra de eventos de participação por papel/evento
	// (host_world_dropin). Vazia no queue_arena. Aditivo (spec 004). Nome distinto
	// do GameStat.Participations (contagem do queue_arena).
	RoleParticipations []ParticipationStat
	// Placement é a distribuição de colocação do battle_royale (BR3, spec 005).
	// nil fora do battle_royale. Aditivo.
	Placement *PlacementDistribution
	// Mission é a distribuição de desfecho da missão do coop_mission (C3, spec 006).
	// nil fora do coop_mission. Aditivo.
	Mission *MissionDistribution
}

// ParticipationEvent é um evento de participação ingerido do fan-out
// (kind=participation_event, session/013). Unidade de trabalho do caminho de
// participação do pool.
type ParticipationEvent struct {
	GameID    string
	SessionID string
	PlayerID  string
	Role      string
	Event     string
	Outcome   string
	// EventID identifica a ocorrência (follow-up #4, spec 007); vazio = data
	// plane antigo, cai no fallback determinístico (ver effectiveEventID).
	EventID string
}

// ParticipationStat é a contagem agregada de um (papel, evento, outcome) num
// jogo (follow-up #5, spec 007: outcome entra na chave, não só um campo solto).
type ParticipationStat struct {
	Role    string
	Event   string
	Outcome string
	Count   int64
}

// MatchRepository é a Port de saída de persistência (rollups em Postgres).
type MatchRepository interface {
	// ApplyMatch agrega uma partida de forma idempotente por matchID (numa tx).
	// dup=true se a partida já havia sido processada (no-op).
	ApplyMatch(ctx context.Context, m MatchResult) (dup bool, err error)
	// ApplyParticipation agrega um evento de participação (host_world_dropin) por
	// (gameID, role, event), idempotente por (sessionID, playerID, event) numa tx.
	// dup=true se o evento já havia sido processado (no-op). Spec 004.
	ApplyParticipation(ctx context.Context, evt ParticipationEvent) (dup bool, err error)
	// ApplyPlacement agrega a distribuição de colocação de uma partida
	// (battle_royale, BR3) numa tx, idempotente por matchID (processed_matches).
	// dup=true se a partida já havia sido processada (no-op). Spec 005.
	ApplyPlacement(ctx context.Context, pr PlacementResult) (dup bool, err error)
	// ApplyCollective agrega o desfecho da missão (coop_mission, C3) numa tx,
	// idempotente por matchID (processed_matches). dup=true se a partida já havia
	// sido processada (no-op). Spec 006.
	ApplyCollective(ctx context.Context, cr CollectiveResult) (dup bool, err error)
	// Overview devolve os totais cross-game + os jogos ordenados por matches desc.
	Overview(ctx context.Context) (Overview, error)
	// GameAnalytics devolve o resumo do jogo + a série diária dos últimos `days`.
	GameAnalytics(ctx context.Context, gameID string, days int) (GameAnalytics, error)
}

// Ingestor é a Port de entrada usada pelo consumidor (driving adapter): entrega
// uma partida ao worker pool e devolve um channel com o resultado (para o
// consumidor deletar a mensagem só no sucesso).
type Ingestor interface {
	Submit(ctx context.Context, m MatchResult) <-chan error
	// SubmitParticipation entrega um evento de participação ao pool (spec 004),
	// pelo mesmo mecanismo de fan-in (channel de retorno) do Submit.
	SubmitParticipation(ctx context.Context, evt ParticipationEvent) <-chan error
	// SubmitPlacement entrega um resultado por colocação ao pool (spec 005), pelo
	// mesmo mecanismo de fan-in do Submit.
	SubmitPlacement(ctx context.Context, pr PlacementResult) <-chan error
	// SubmitCollective entrega um desfecho de missão ao pool (spec 006), pelo
	// mesmo mecanismo de fan-in do Submit.
	SubmitCollective(ctx context.Context, cr CollectiveResult) <-chan error
}
