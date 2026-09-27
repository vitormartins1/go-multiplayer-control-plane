// Package service contém a regra de negócio do achievements-service.
// Só depende das interfaces de domain (ADR-0002). Tudo escopado por gameID
// (multi-tenant, ADR-0017).
package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/achievements-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos as Ports de entrada.
var _ domain.AchievementService = (*Service)(nil)
var _ domain.EventRuleService = (*Service)(nil)

// slugRE valida o id (slug) de uma conquista: 2–40, minúsculas/dígitos/hífen/
// underscore, sem separador nas pontas. Permite o estilo snake_case do catálogo
// (ex.: "first_win", "ten_wins").
var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,38}[a-z0-9]$`)

// statKeyRE valida o stat_key de um critério por métrica (mesma forma dos
// scoringStats do perfil de multiplayer — ADR-0023).
var statKeyRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// steamAPINameRE é o formato do "API Name" de conquista no Steamworks (spec 019).
var steamAPINameRE = regexp.MustCompile(`^[A-Za-z0-9_]{1,128}$`)

const maxAchievementNameLen = 100

// vocabulário do outcome no envelope (ADR-0023). A tradução outcome/stats ->
// event_type do catálogo é feita por domain.EvaluateRules (motor de event
// rules, ADR-0028) — não por constantes fixas aqui.
const (
	outcomeWin  = "win"
	outcomeLoss = "loss"
	outcomeDraw = "draw"
)

// Service implementa as regras de catálogo e desbloqueio de conquistas.
type Service struct {
	repo  domain.AchievementRepository
	rules domain.EventRuleRepository
	now   func() time.Time
}

// New cria o service a partir do repositório de catálogo/progresso e do
// repositório de regras de mapeamento evento->achievement (migration 008).
func New(repo domain.AchievementRepository, rules domain.EventRuleRepository) *Service {
	return &Service{repo: repo, rules: rules, now: time.Now}
}

// ListCatalog devolve o catálogo de conquistas do jogo.
func (s *Service) ListCatalog(ctx context.Context, gameID string) ([]domain.Achievement, error) {
	cat, err := s.repo.ListCatalog(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("achievements service: list catalog: %w", err)
	}
	return cat, nil
}

// ListForPlayer devolve o catálogo do jogo anotado com o estado de desbloqueio
// do jogador nesse jogo.
func (s *Service) ListForPlayer(ctx context.Context, gameID, playerID string) ([]domain.PlayerAchievement, error) {
	cat, err := s.repo.ListCatalog(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("achievements service: list for player: %w", err)
	}
	progress, err := s.repo.ListPlayerProgress(ctx, gameID, playerID)
	if err != nil {
		return nil, fmt.Errorf("achievements service: list for player: %w", err)
	}

	byID := make(map[string]domain.PlayerProgress, len(progress))
	for _, p := range progress {
		byID[p.AchievementID] = p
	}

	out := make([]domain.PlayerAchievement, 0, len(cat))
	for _, a := range cat {
		pa := domain.PlayerAchievement{Achievement: a}
		if p, ok := byID[a.ID]; ok {
			pa.Progress = min(p.Progress, a.Target) // nunca exibir além da meta
			if !p.UnlockedAt.IsZero() {
				pa.Unlocked = true
				pa.UnlockedAt = p.UnlockedAt
			}
		}
		out = append(out, pa)
	}
	return out, nil
}

// RecordEvent avalia o evento contra o catálogo do jogo e desbloqueia as
// conquistas cujo critério (event_type) casa, de forma idempotente. Retorna
// apenas as recém-desbloqueadas nesta chamada.
func (s *Service) RecordEvent(ctx context.Context, gameID, playerID string, event domain.Event) ([]domain.Achievement, error) {
	if event.Type == "" {
		return nil, domain.ErrInvalidEvent
	}

	matches, err := s.repo.FindByEventType(ctx, gameID, event.Type)
	if err != nil {
		return nil, fmt.Errorf("achievements service: record event: %w", err)
	}

	at := s.now().UTC()
	var unlocked []domain.Achievement
	for _, a := range matches {
		justUnlocked, err := s.repo.RecordProgress(ctx, gameID, playerID, a.ID, a.Target, at)
		if err != nil {
			return nil, fmt.Errorf("achievements service: record progress %q: %w", a.ID, err)
		}
		if justUnlocked {
			unlocked = append(unlocked, a)
		}
	}
	return unlocked, nil
}

// ApplyMatchResult traduz o resultado bruto da partida (queue_arena,
// win_loss_draw) em eventos de gameplay via EvaluateRules (migration 008) e
// delega ao repo a aplicação idempotente por matchID. Consumido pelo fan-out
// durável (SQS, ADR-0020). dup=true quando a partida já havia sido processada
// (no-op).
func (s *Service) ApplyMatchResult(ctx context.Context, gameID, matchID, modeID string, results []domain.MatchResult) (bool, error) {
	rules, err := s.rules.RulesForArchetype(ctx, gameID, domain.ArchetypeQueueArena)
	if err != nil {
		return false, fmt.Errorf("achievements service: apply match result: rules: %w", err)
	}
	players := make([]domain.PlayerMatchInput, 0, len(results))
	for _, r := range results {
		outcome := r.Outcome
		if outcome == "" {
			// fallback retrocompat: envelope sem outcome (v1) -> deriva de Won.
			// Won é bool estrito nesse formato (sem conceito de empate — ver
			// domain.MatchResult), então false representa derrota de verdade,
			// não "sem informação" (achado no /code-review: só o ramo Won=true
			// setava outcome, deixando derrotas v1 com outcome="" e nunca
			// casando regras `outcome=loss`).
			if r.Won {
				outcome = outcomeWin
			} else {
				outcome = outcomeLoss
			}
		}
		events := domain.EvaluateRules(rules, gameID,
			domain.WinLossDrawFacts(outcome).WithMode(modeID).WithStats(r.Stats))
		players = append(players, domain.PlayerMatchInput{PlayerID: r.PlayerID, EventTypes: events, Stats: r.Stats})
	}
	dup, err := s.repo.ApplyMatchResult(ctx, gameID, matchID, players, s.now().UTC())
	if err != nil {
		return false, fmt.Errorf("achievements service: apply match result: %w", err)
	}
	return dup, nil
}

// ApplyPlacementResult traduz o resultado por colocação (battle_royale, BR3)
// em event types de gameplay via EvaluateRules (migration 008) e stats
// (survived_ms exposto p/ os critérios por-métrica), delegando ao repo a
// aplicação idempotente por matchID. Consumido pelo fan-out durável
// (kind=placement, session/015). dup=true quando a partida já havia sido
// processada (no-op).
func (s *Service) ApplyPlacementResult(ctx context.Context, gameID, matchID, modeID string, entries []domain.PlacementEntry) (bool, error) {
	rules, err := s.rules.RulesForArchetype(ctx, gameID, domain.ArchetypeBattleRoyale)
	if err != nil {
		return false, fmt.Errorf("achievements service: apply placement result: rules: %w", err)
	}
	players := make([]domain.PlayerMatchInput, 0, len(entries))
	for _, e := range entries {
		// o mesmo mapa (com survived_ms injetado) alimenta o critério por
		// métrica E os facts stat.* — regra `stat.survived_ms >= N` funciona.
		stats := withSurvivedMs(e.Stats, e.SurvivedMs)
		players = append(players, domain.PlayerMatchInput{
			PlayerID: e.PlayerID,
			EventTypes: domain.EvaluateRules(rules, gameID,
				domain.PlacementFacts(e.Placement).WithMode(modeID).WithStats(stats)),
			Stats: stats,
		})
	}
	dup, err := s.repo.ApplyMatchResult(ctx, gameID, matchID, players, s.now().UTC())
	if err != nil {
		return false, fmt.Errorf("achievements service: apply placement result: %w", err)
	}
	return dup, nil
}

// withSurvivedMs devolve os stats com survived_ms injetado (a sobrevivência é um
// campo de topo do placement; expô-la como stat deixa os critérios por-métrica
// mirá-la). Não sobrescreve um survived_ms já presente no mapa.
func withSurvivedMs(stats map[string]int64, survivedMs int64) map[string]int64 {
	out := make(map[string]int64, len(stats)+1)
	for k, v := range stats {
		out[k] = v
	}
	if _, ok := out[domain.StatSurvivedMs]; !ok {
		out[domain.StatSurvivedMs] = survivedMs
	}
	return out
}

// ApplyCollectiveResult traduz o desfecho COLETIVO da missão (coop_mission, C3)
// em event types de gameplay (played p/ todos; success/failed conforme o desfecho
// da SESSÃO, via CollectiveEventTypes) e stats por jogador, delegando ao repo a
// aplicação idempotente por matchID. Consumido pelo fan-out durável
// (kind=collective, session/016). dup=true quando a partida já havia sido
// processada (no-op). Espelha ApplyPlacementResult.
func (s *Service) ApplyCollectiveResult(ctx context.Context, gameID, matchID, modeID, outcome string, players []domain.CollectivePlayer) (bool, error) {
	rules, err := s.rules.RulesForArchetype(ctx, gameID, domain.ArchetypeCoopMission)
	if err != nil {
		return false, fmt.Errorf("achievements service: apply collective result: rules: %w", err)
	}
	// Avaliação POR JOGADOR (spec 015): o desfecho é coletivo, mas os stats
	// são individuais — uma regra com stat.<x> pode casar só pro MVP da
	// missão. Sem stat.* nas regras, o resultado é idêntico ao de antes
	// (facts iguais por jogador), só que uniforme com os outros kinds.
	inputs := make([]domain.PlayerMatchInput, 0, len(players))
	for _, p := range players {
		inputs = append(inputs, domain.PlayerMatchInput{
			PlayerID: p.PlayerID,
			EventTypes: domain.EvaluateRules(rules, gameID,
				domain.CollectiveFacts(outcome).WithMode(modeID).WithStats(p.Stats)),
			Stats: p.Stats,
		})
	}
	dup, err := s.repo.ApplyMatchResult(ctx, gameID, matchID, inputs, s.now().UTC())
	if err != nil {
		return false, fmt.Errorf("achievements service: apply collective result: %w", err)
	}
	return dup, nil
}

// ApplyParticipationEvent traduz um evento de participação do host_world_dropin
// (spec 013/009) nos event types de gameplay por papel e delega ao repo a
// aplicação idempotente por eventID (follow-up #4, spec 012). Consumido pelo
// fan-out durável (SQS). dup=true quando o evento já havia sido processado.
// stats (spec 018) soma facts stat.* pra regras com comparador E avança
// achievements por métrica — mesmo tratamento que ApplyMatchResult já dá aos
// outros 3 arquétipos; nil/vazio é no-op (compat total com envelopes antigos).
func (s *Service) ApplyParticipationEvent(ctx context.Context, gameID, sessionID, playerID, modeID, role, event, outcome, eventID string, stats map[string]int64) (bool, error) {
	rules, err := s.rules.RulesForArchetype(ctx, gameID, domain.ArchetypeHostWorldDropIn)
	if err != nil {
		return false, fmt.Errorf("achievements service: apply participation event: rules: %w", err)
	}
	facts := domain.ParticipationFacts(role, event, outcome).WithMode(modeID).WithStats(stats)
	eventTypes := domain.EvaluateRules(rules, gameID, facts)
	dup, err := s.repo.ApplyParticipationEvent(ctx, gameID, sessionID, playerID, event, eventID, eventTypes, stats, s.now().UTC())
	if err != nil {
		return false, fmt.Errorf("achievements service: apply participation event: %w", err)
	}
	return dup, nil
}

// ProgressForAchievement devolve o progresso de todos os jogadores na conquista.
func (s *Service) ProgressForAchievement(ctx context.Context, gameID, achievementID string) ([]domain.AchievementProgressEntry, error) {
	entries, err := s.repo.ProgressForAchievement(ctx, gameID, achievementID)
	if err != nil {
		return nil, fmt.Errorf("achievements service: progress for achievement: %w", err)
	}
	return entries, nil
}

// validateCriterion normaliza e valida name/target e o critério (evento OU stat,
// mutuamente exclusivos — ADR-0023 A1). Opera sobre *a in-place (default de
// target). ErrInvalidAchievement em entrada inválida.
func validateCriterion(a *domain.Achievement) error {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" || len(a.Name) > maxAchievementNameLen {
		return domain.ErrInvalidAchievement
	}
	if a.Target < 1 {
		a.Target = 1
	}
	// spec 019: API Name do Steamworks — letras, dígitos e _, até 128.
	a.SteamAPIName = strings.TrimSpace(a.SteamAPIName)
	if a.SteamAPIName != "" && !steamAPINameRE.MatchString(a.SteamAPIName) {
		return domain.ErrInvalidAchievement
	}
	if a.StatKey != "" {
		// critério por stat: stat_key/stat_mode válidos, event_type ausente.
		if a.EventType != "" || !statKeyRE.MatchString(a.StatKey) {
			return domain.ErrInvalidAchievement
		}
		if a.StatMode != domain.StatModeCumulative && a.StatMode != domain.StatModePerMatch {
			return domain.ErrInvalidAchievement
		}
		return nil
	}
	// critério por evento: event_type obrigatório; stat_mode não se aplica.
	if a.EventType == "" || a.StatMode != "" {
		return domain.ErrInvalidAchievement
	}
	return nil
}

// CreateAchievement valida e adiciona uma conquista ao catálogo do jogo (admin).
func (s *Service) CreateAchievement(ctx context.Context, gameID string, a domain.Achievement) (*domain.Achievement, error) {
	if !slugRE.MatchString(a.ID) {
		return nil, domain.ErrInvalidAchievement
	}
	if err := validateCriterion(&a); err != nil {
		return nil, err
	}
	a.GameID = gameID

	if err := s.repo.CreateAchievement(ctx, &a); err != nil {
		return nil, fmt.Errorf("achievements service: create achievement: %w", err)
	}
	return &a, nil
}

// UpdateAchievement edita os campos mutáveis da conquista (id/gameID são a
// chave). Valida name/critério como na criação; o slug não é revalidado
// (vem do path, já existente).
func (s *Service) UpdateAchievement(ctx context.Context, gameID, id string, a domain.Achievement) (*domain.Achievement, error) {
	if err := validateCriterion(&a); err != nil {
		return nil, err
	}
	a.GameID = gameID
	a.ID = id

	if err := s.repo.UpdateAchievement(ctx, &a); err != nil {
		return nil, fmt.Errorf("achievements service: update achievement: %w", err)
	}
	return &a, nil
}

// DeleteAchievement remove a conquista e o progresso dos jogadores nela.
func (s *Service) DeleteAchievement(ctx context.Context, gameID, id string) error {
	if err := s.repo.DeleteAchievement(ctx, gameID, id); err != nil {
		return fmt.Errorf("achievements service: delete achievement: %w", err)
	}
	return nil
}

// validArchetypes é o vocabulário de arquétipos que o achievements-service
// sabe avaliar (um normalizador de facts em event_rule.go por arquétipo).
var validArchetypes = map[string]bool{
	domain.ArchetypeQueueArena:      true,
	domain.ArchetypeHostWorldDropIn: true,
	domain.ArchetypeBattleRoyale:    true,
	domain.ArchetypeCoopMission:     true,
}

// eventTypeRE valida cada item de EventTypes de uma regra — mesma forma do
// stat_key/slug já usada no resto do catálogo.
var eventTypeRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// validateEventRule normaliza e valida uma regra (admin). Match nil vira mapa
// vazio (regra "sempre bate", ex.: o "played" do seed) — não é erro, só um
// caso legítimo. ErrInvalidEventRule em archetype desconhecido, eventTypes
// vazio/malformado ou condição malformada (spec 014: op desconhecido, value
// de tipo errado ou lista vazia no `in` — a validação ESTRITA que o decode
// leniente do MatchCond delega pra cá).
func validateEventRule(r *domain.EventRule) error {
	if !validArchetypes[r.Archetype] {
		return domain.ErrInvalidEventRule
	}
	return validateEventRuleContent(r)
}

// validateEventRuleContent valida só match+eventTypes — a parte que o UPDATE
// (spec 016) compartilha com o create (archetype não é editável, então o
// update não recebe/valida arquétipo).
func validateEventRuleContent(r *domain.EventRule) error {
	if len(r.EventTypes) == 0 {
		return domain.ErrInvalidEventRule
	}
	for _, et := range r.EventTypes {
		if !eventTypeRE.MatchString(et) {
			return domain.ErrInvalidEventRule
		}
	}
	if r.Match == nil {
		r.Match = map[string]domain.MatchCond{}
	}
	for k, c := range r.Match {
		if strings.TrimSpace(k) == "" || !c.Valid() {
			return domain.ErrInvalidEventRule
		}
	}
	return nil
}

// ListEffectiveRules devolve as regras globais de cada arquétipo + as
// específicas do jogo, de todos os arquétipos (admin, painel de Event Rules).
func (s *Service) ListEffectiveRules(ctx context.Context, gameID string) ([]domain.EventRule, error) {
	rules, err := s.rules.ListEffective(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("achievements service: list effective rules: %w", err)
	}
	return rules, nil
}

// CreateEventRule cria uma regra escopada ao jogo (GameID sempre setado pelo
// service — nunca aceito de outro valor do payload).
func (s *Service) CreateEventRule(ctx context.Context, gameID string, r domain.EventRule) (*domain.EventRule, error) {
	if err := validateEventRule(&r); err != nil {
		return nil, err
	}
	r.GameID = &gameID
	if err := s.rules.CreateRule(ctx, &r); err != nil {
		return nil, fmt.Errorf("achievements service: create event rule: %w", err)
	}
	return &r, nil
}

// UpdateEventRule substitui match/eventTypes de uma regra do jogo (spec 016).
// GameID e ID vêm do path (nunca do payload); archetype é preservado pelo
// repositório (RETURNING) — não é editável.
func (s *Service) UpdateEventRule(ctx context.Context, gameID, id string, r domain.EventRule) (*domain.EventRule, error) {
	if err := validateEventRuleContent(&r); err != nil {
		return nil, err
	}
	r.ID = id
	r.GameID = &gameID
	if err := s.rules.UpdateRule(ctx, &r); err != nil {
		return nil, fmt.Errorf("achievements service: update event rule: %w", err)
	}
	return &r, nil
}

// DeleteEventRule remove uma regra do jogo (nunca uma global — a Port de
// saída garante isso via WHERE game_id = $2, que nunca casa NULL).
func (s *Service) DeleteEventRule(ctx context.Context, gameID, id string) error {
	if err := s.rules.DeleteRule(ctx, gameID, id); err != nil {
		return fmt.Errorf("achievements service: delete event rule: %w", err)
	}
	return nil
}

// SuppressEventRule desliga uma regra global para o jogo gameID (spec 017,
// V2b). Passthrough fino — gameID/id vêm do path, sem conteúdo de payload
// pra validar (ao contrário de Create/Update); a checagem de "existe e é
// global" mora no repositório.
func (s *Service) SuppressEventRule(ctx context.Context, gameID, id string) error {
	if err := s.rules.Suppress(ctx, gameID, id); err != nil {
		return fmt.Errorf("achievements service: suppress event rule: %w", err)
	}
	return nil
}

// UnsuppressEventRule restaura uma regra global suprimida (spec 017, V2b).
func (s *Service) UnsuppressEventRule(ctx context.Context, gameID, id string) error {
	if err := s.rules.Unsuppress(ctx, gameID, id); err != nil {
		return fmt.Errorf("achievements service: unsuppress event rule: %w", err)
	}
	return nil
}
