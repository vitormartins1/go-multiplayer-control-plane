// Package domain contém os tipos puros e as Ports do analytics-service. Zero
// dependências externas (ADR-0002). Tudo escopado por gameID (ADR-0017).
package domain

import (
	"context"
	"time"
)

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
	// Stats são métricas opcionais do jogador (kills, assists, ...; spec
	// 011) — publicadas no envelope desde a origem, descartadas aqui até
	// esta spec. Nil/vazio = jogo sem scoringStats (sem regressão).
	Stats map[string]int64
}

// MatchResult é o resultado de uma partida ingerido do fan-out (ADR-0020/0021).
// É a unidade de trabalho do worker pool.
type MatchResult struct {
	MatchID string
	GameID  string
	// ModeID é o modo do jogo (ADR-0024) — publicado no envelope desde a
	// spec 010, mas só extraído aqui na 012 (gap: os "primos"
	// PlacementResult/CollectiveResult/ParticipationEvent já tinham).
	// "" = evento anterior a esta correção (dado legado).
	ModeID  string
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
	// ModeID é o modo do jogo (ADR-0024) — publicado no envelope desde a
	// origem, descartado aqui até a spec 010 (mesma lacuna que a 009 fechou
	// pra participação).
	ModeID  string
	Entries []PlacementEntry
}

// PlacementEntry é a colocação de um participante. O analytics agrega
// distribuição, sobrevivência e (spec 011) stats brutos por jogador.
type PlacementEntry struct {
	// PlayerID identifica o jogador — usado só pra gravar stats brutos (spec
	// 011); o histograma de colocação continua agregado, sem guardar quem é
	// quem.
	PlayerID   string
	Placement  int
	SurvivedMs int64
	Stats      map[string]int64 // spec 011; nil/vazio = sem scoringStats
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
// fan-out (kind=collective, session/016). O analytics agrega o desfecho da
// SESSÃO (rollup) e, por jogador (spec 011), os stats brutos.
type CollectiveResult struct {
	MatchID string
	GameID  string
	// ModeID é o modo do jogo (ADR-0024) — mesma lacuna de PlacementResult,
	// fechada na spec 010.
	ModeID  string
	Outcome string // MISSION_SUCCESS | MISSION_FAILURE
	// Players carrega os stats brutos por jogador (spec 011) — o desfecho
	// continua sendo só o Outcome da sessão (não por jogador). Vazio = envelope
	// antigo ou jogo sem scoringStats.
	Players []CollectivePlayerStat
}

// CollectivePlayerStat é a participação de um jogador na missão, só pra
// captura de stats brutos (spec 011) — o desfecho é coletivo (Outcome acima),
// não por jogador.
type CollectivePlayerStat struct {
	PlayerID string
	Stats    map[string]int64
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
	Sessions      int64 // sessões/mundos fechados, todos os arquétipos (spec 016)
	Matches       int64 // partidas de queue_arena
	UniquePlayers int64 // distintos globais
	Games         int64 // jogos com atividade
}

// OverviewGame é a linha de um jogo no overview cross-game (spec 016): a
// medida comum a todos os arquétipos é a SESSÃO fechada (game_session_rollup);
// Matches/Wins/... seguem vindo do rollup de queue_arena.
type OverviewGame struct {
	GameStat
	Sessions     int64
	AvgSessionMs int64 // duração média das sessões fechadas
	Abandoned    int64 // fechadas por abandoned (server morreu) ou ttl (ninguém reportou)
}

// SessionsPoint é um ponto da série "sessões fechadas por dia".
type SessionsPoint struct {
	Day      string // YYYY-MM-DD (UTC)
	Sessions int64
}

// Overview é o panorama cross-game (aba Analytics): totais, jogos e a série de
// sessões fechadas por dia somada entre todos os jogos (spec 016 — antes era
// "partidas por dia", que só existia para queue_arena).
type Overview struct {
	Totals Totals
	Games  []OverviewGame
	Daily  []SessionsPoint
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
	// ModeID é o filtro efetivamente aplicado (spec 012); "" = todos os
	// modos. Eco do parâmetro de entrada — o cliente confirma o que
	// recebeu sem adivinhar pelo conteúdo (séries vazias podem ser "sem
	// dado" OU "filtro não bateu com nenhum modo", indistinguíveis sem o eco).
	ModeID string
	Daily  []DailyPoint
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

	// Séries diárias dos 3 arquétipos que não tinham (spec 010) — mesmo
	// padrão nil-able de Placement/Mission: vazia fora do arquétipo
	// correspondente. Soma todos os modos (sem seletor nesta v1).
	PlacementDaily     []DailyPoint // battle_royale
	MissionDaily       []DailyPoint // coop_mission
	ParticipationDaily []DailyPoint // host_world_dropin (conta EVENTOS, não sessões)

	// Stats é o resumo agregado por stat_key (spec 011) — soma/contagem/média
	// entre TODOS os jogadores e partidas do jogo. Vazio = jogo sem
	// scoringStats (nunca reportou stats). Não expõe valor por jogador (isso
	// é papel do leaderboard-service).
	Stats []StatSummary

	// SessionHealth é a distribuição por motivo de fechamento (spec 013) —
	// já filtrada por modeID quando aplicável (a tabela de origem nasce COM
	// mode_id, sem a lacuna do item D que afeta os demais campos deste
	// struct). Vazio = jogo sem nenhum session_closed processado ainda.
	SessionHealth []SessionCloseStat

	// SessionCloseDaily é a série "sessões fechadas por dia" (spec 015) —
	// total por dia, todos os motivos somados (o detalhe por motivo já
	// está em SessionHealth). Mesmo padrão das outras 3 séries diárias.
	SessionCloseDaily []DailyPoint
}

// StatSummary é o panorama agregado de uma métrica do jogo (spec 011): soma,
// número de ocorrências (jogador×partida) e média derivada.
type StatSummary struct {
	Key   string
	Sum   int64
	Count int64
	Avg   float64
}

// SessionCloseEvent é o fechamento de uma sessão, qualquer arquétipo/motivo
// (session-service/028, kind=session_closed). Unidade de trabalho do
// caminho de "session health" do pool.
type SessionCloseEvent struct {
	SessionID   string
	GameID      string
	ModeID      string
	Archetype   string
	CloseReason string
	OpenedAt    time.Time
	ClosedAt    time.Time
}

// SessionCloseStat é a distribuição agregada por motivo de fechamento (spec
// 013) — conta + duração total. Avg é computado no read (DurationSumMs /
// Count), mesmo padrão de StatSummary.Avg.
type SessionCloseStat struct {
	ModeID        string
	Archetype     string
	CloseReason   string
	Count         int64
	DurationSumMs int64
	AvgDurationMs float64
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
	// ModeID é o modo nomeado do mundo (ADR-0024, spec 009); "" = evento
	// anterior a essa dimensão existir (bucket "sem modo").
	ModeID string
	// Stats são métricas opcionais do participante (spec 011, destravado por
	// session-service/026). Nil/vazio = data plane antigo ou sem scoringStats.
	Stats map[string]int64
}

// ParticipationStat é a contagem agregada de um (modo, papel, evento, outcome)
// num jogo (spec 009: modeId entra na chave — follow-up #5/spec 007 já tinha
// feito o mesmo com outcome).
type ParticipationStat struct {
	ModeID  string
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
	// ApplySessionClosed agrega um fechamento de sessão (spec 013),
	// idempotente por sessionID (processed_sessions) numa tx. dup=true se
	// já processado (no-op).
	ApplySessionClosed(ctx context.Context, evt SessionCloseEvent) (dup bool, err error)
	// Overview devolve os totais cross-game + os jogos ordenados por matches desc.
	Overview(ctx context.Context) (Overview, error)
	// GameAnalytics devolve o resumo do jogo + a série diária dos últimos
	// `days`. modeID == "" agrega TODOS os modos (comportamento atual,
	// default); modeID != "" filtra as séries mode-scoped (spec 012).
	// Matches/Participations/Wins/Losses/Draws/AvgScore/Placement/Mission
	// NUNCA são filtrados por modo (tabelas sem essa dimensão — item D,
	// fora de escopo).
	GameAnalytics(ctx context.Context, gameID string, days int, modeID string) (GameAnalytics, error)
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
	// SubmitSessionClosed entrega um fechamento de sessão ao pool (spec 013),
	// pelo mesmo mecanismo de fan-in do Submit.
	SubmitSessionClosed(ctx context.Context, evt SessionCloseEvent) <-chan error
}
