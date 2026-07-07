package domain

import "context"

// Arquétipo cujo perfil o matchmaking sabe invadir (ADR-0023, spec 006).
// Vocabulário do game-registry.
const archetypeHostWorldDropIn = "host_world_dropin"

// InvaderCriteria são os atributos que o invasor traz na chamada Invade (spec
// 006): a faixa dele (soul level / weapon tier), sobre a qual a elegibilidade é
// calculada contra cada mundo.
type InvaderCriteria struct {
	SoulLevel  int
	WeaponTier int
}

// InvadableWorld espelha o diretório do session (session/012): um mundo aberto
// com vaga de invasor e os atributos do host. Fonte de dados pura — a política
// de faixa é aplicada aqui no matchmaking (Eligible), não no session.
type InvadableWorld struct {
	SessionID        string
	HostSoulLevel    int
	WeaponTier       int
	FreeInvaderSlots int
}

// InvasionPolicy é o subconjunto do perfil host_world_dropin que decide se e como
// a invasão casa (spec 006): o arquétipo e as flags que ligam/desligam cada
// filtro. Lido do game-registry via InvasionPolicyProvider.
type InvasionPolicy struct {
	Archetype         string
	InvasionEnabled   bool
	LevelRangeEnabled bool
	WeaponTierEnabled bool
}

// SupportsInvasion indica se o jogo aceita invasão: é host_world_dropin com a
// invasão ligada. Fora disso, Invade rejeita com ErrInvasionNotSupported.
func (p InvasionPolicy) SupportsInvasion() bool {
	return p.Archetype == archetypeHostWorldDropIn && p.InvasionEnabled
}

// Injected é o que o WorldInjector devolve ao injetar o invasor num mundo (spec
// 006): o endereço do server e o connect token (claim role=invader).
type Injected struct {
	ServerAddress string
	ConnectToken  string
}

// Invaded é o resultado da operação Invade para o cliente: o mundo em que o
// invasor entrou e como conectar (server + token).
type Invaded struct {
	SessionID     string
	ServerAddress string
	ConnectToken  string
}

// Constantes tunáveis da banda de elegibilidade (spec 006, D5). Números
// ilustrativos da ficha 01 §4 — a invasão "punge para cima" (banda assimétrica):
// o invasor casa hosts num pouco abaixo até bem acima do seu nível. NÃO são os
// valores exatos da FromSoftware; ajustáveis sem tocar a lógica.
const (
	// belowFraction: quanto ABAIXO do nível do invasor um host ainda casa
	// (fração do nível do invasor). Ex.: 0.1 → invasor L=100 casa hosts >= 90.
	belowFraction = 0.10
	// aboveFraction/aboveFlat: quanto ACIMA. Assimétrico (above > below): um
	// invasor entra em mundos mais fortes que o dele. Ex.: L=100 → +10 +20 = 130.
	aboveFraction = 0.10
	aboveFlat     = 20
	// weaponTierBand: diferença máxima de weapon tier tolerada quando o filtro
	// de tier está ligado.
	weaponTierBand = 1
)

// Eligible decide se um mundo é invadível pelo invasor, conforme a política
// (spec 006, D5). Função pura e testável — o coração da elegibilidade por faixa.
//
//   - LevelRangeEnabled=false: ignora o nível (casa qualquer mundo com vaga).
//   - LevelRangeEnabled=true: banda ASSIMÉTRICA em torno do nível do invasor.
//     Host H casa invasor L sse H ∈ [L - below·L, L + above·L + aboveFlat].
//     Mundo sem atributos (host soul level 0) NÃO casa — não dá para casar por
//     faixa sem o nível do host.
//   - WeaponTierEnabled=true: exige |hostTier - invaderTier| <= weaponTierBand.
func Eligible(crit InvaderCriteria, w InvadableWorld, p InvasionPolicy) bool {
	if p.LevelRangeEnabled {
		if w.HostSoulLevel <= 0 {
			return false // mundo sem faixa não é casável por faixa
		}
		lo := crit.SoulLevel - int(belowFraction*float64(crit.SoulLevel))
		hi := crit.SoulLevel + int(aboveFraction*float64(crit.SoulLevel)) + aboveFlat
		if w.HostSoulLevel < lo || w.HostSoulLevel > hi {
			return false
		}
	}
	if p.WeaponTierEnabled {
		diff := w.WeaponTier - crit.WeaponTier
		if diff < 0 {
			diff = -diff
		}
		if diff > weaponTierBand {
			return false
		}
	}
	return true
}

// Invader é a Port de entrada do segundo modo do matchmaking (spec 006):
// descobrir um mundo elegível e injetar o invasor nele. Stateless — sem fila,
// sem match: uma chamada resolve na hora ou falha.
type Invader interface {
	// Invade lê a política do jogo, lista os mundos invadíveis, filtra por faixa
	// e injeta o invasor no primeiro elegível. ErrInvasionNotSupported (arquétipo
	// errado / invasão off), ErrNoEligibleWorld (nada casa ou todos cheios),
	// ErrProfileUnavailable (perfil ilegível). O playerID vem do JWT (context).
	Invade(ctx context.Context, gameID, playerID string, crit InvaderCriteria) (Invaded, error)
}

// InvasionPolicyProvider é a Port de saída que lê a política de invasão do jogo
// no game-registry (spec 006). Separada do ProfileProvider (queue_arena) para
// não tocar o cache do caminho crítico do FindMatch. ErrProfileUnavailable em
// falha de leitura.
type InvasionPolicyProvider interface {
	InvasionPolicy(ctx context.Context, gameID string) (InvasionPolicy, error)
}

// WorldDirectory é a Port de saída que lista os mundos invadíveis de um jogo no
// session-service (session/012). Fotografia dos mundos com vaga de invasor.
type WorldDirectory interface {
	ListInvadableWorlds(ctx context.Context, gameID string) ([]InvadableWorld, error)
}

// WorldInjector é a Port de saída que injeta o invasor num mundo via o JoinWorld
// do session (spec 006). ErrRoleFull (mundo encheu na corrida → retry no
// próximo), ErrWorldNotFound (mundo fechou entre listar e injetar).
type WorldInjector interface {
	Inject(ctx context.Context, sessionID, playerID, role string) (Injected, error)
}
