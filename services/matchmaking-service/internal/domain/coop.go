package domain

import (
	"context"
	"time"
)

// Sign é um sinal de invocação no quadro de co-op (spec 007): um aliado anuncia
// disponibilidade para ajudar, com a sua faixa. Dado puro — o canal de entrega e
// o estado (active/claimed) vivem no store interno do serviço, não aqui.
type Sign struct {
	SignID     string
	GameID     string
	PlayerID   string // o aliado que anunciou
	SoulLevel  int
	WeaponTier int
	PostedAt   time.Time
}

// Summoned é o que o aliado recebe quando um host o invoca (spec 007): o mundo em
// que entrou como coop e como conectar. Entregue pelo canal do PostSummonSign.
type Summoned struct {
	SessionID     string
	ServerAddress string
	ConnectToken  string
}

// CoopCriteria é a faixa que o aliado traz ao anunciar o sinal — e, do outro
// lado, a faixa do mundo do host ao listar (a compatibilidade é calculada entre
// as duas). Espelha o InvaderCriteria da invasão.
type CoopCriteria struct {
	SoulLevel  int
	WeaponTier int
}

// CoopPolicy é o subconjunto do perfil host_world_dropin que decide se e como o
// co-op casa (spec 007): o arquétipo, se o papel coop está habilitado, e as flags
// de faixa. Irmã da InvasionPolicy (a invasão tem a sua). Lido via CoopPolicyProvider.
type CoopPolicy struct {
	Archetype         string
	CoopEnabled       bool
	LevelRangeEnabled bool
	WeaponTierEnabled bool
}

// SupportsCoop indica se o jogo aceita co-op: é host_world_dropin com o papel
// coop habilitado. Fora disso, o quadro de sinais rejeita com ErrCoopNotSupported.
func (p CoopPolicy) SupportsCoop() bool {
	return p.Archetype == archetypeHostWorldDropIn && p.CoopEnabled
}

// Constantes tunáveis da banda de co-op (spec 007, ficha 01 §4: co-op ≈ X ± (10 +
// 10%)). Ao contrário da invasão (assimétrica), a banda do co-op é SIMÉTRICA em
// torno do nível do aliado. Números ilustrativos, ajustáveis sem tocar a lógica.
const (
	coopFlat     = 10
	coopFraction = 0.10
)

// CoopEligible decide se o mundo de um host (faixa hostSoulLevel/hostWeaponTier) é
// compatível com um sinal de co-op (ally), conforme a política (spec 007, D5).
// Função pura, testável — irmã simétrica de Eligible (invasão).
//
//   - LevelRangeEnabled=false: ignora o nível (todo sinal ativo casa).
//   - LevelRangeEnabled=true: banda SIMÉTRICA em torno do nível do aliado A.
//     Host H casa sse H ∈ [A - (coopFlat + coopFraction·A), A + (coopFlat + coopFraction·A)].
//     Sinal ou host sem nível (0) NÃO casa — não dá para casar por faixa.
//   - WeaponTierEnabled=true: exige |hostWeaponTier - ally.WeaponTier| <= weaponTierBand.
func CoopEligible(ally CoopCriteria, hostSoulLevel, hostWeaponTier int, p CoopPolicy) bool {
	if p.LevelRangeEnabled {
		if ally.SoulLevel <= 0 || hostSoulLevel <= 0 {
			return false
		}
		band := coopFlat + int(coopFraction*float64(ally.SoulLevel))
		if hostSoulLevel < ally.SoulLevel-band || hostSoulLevel > ally.SoulLevel+band {
			return false
		}
	}
	if p.WeaponTierEnabled {
		diff := hostWeaponTier - ally.WeaponTier
		if diff < 0 {
			diff = -diff
		}
		if diff > weaponTierBand { // weaponTierBand: reusa a const da invasão (invasion.go)
			return false
		}
	}
	return true
}

// Summoner é a Port de entrada do terceiro modo do matchmaking (spec 007): o
// quadro de sinais de co-op, em dois passos (anúncio → escolha do host → injeção).
type Summoner interface {
	// PostSign publica um sinal do aliado e devolve o signID e um canal por onde
	// o Summoned chega quando um host o invoca (server-streaming: o aliado aguarda).
	// O canal é fechado sem valor se o sinal expira/cancela (o handler manda EXPIRED).
	// ErrCoopNotSupported se o jogo não aceita co-op. O playerID vem do JWT.
	PostSign(ctx context.Context, gameID, playerID string, crit CoopCriteria) (signID string, updates <-chan Summoned, err error)
	// ListSigns devolve os sinais ativos do jogo compatíveis com o mundo do host
	// (faixa de co-op). Lista vazia se nada casa. ErrCoopNotSupported se o jogo não
	// aceita co-op.
	ListSigns(ctx context.Context, gameID string, host CoopCriteria) ([]Sign, error)
	// Summon injeta o aliado do sinal no mundo sessionID como coop, consome o sinal
	// (no máximo uma vez) e entrega o Summoned ao aliado pelo canal. Devolve o
	// playerID do aliado invocado. ErrSignNotFound, ErrSignConsumed, ErrRoleFull,
	// ErrWorldNotFound.
	Summon(ctx context.Context, sessionID, signID string) (allyID string, err error)
	// CancelSign remove um sinal do quadro (o aliado fechou o stream). Idempotente.
	CancelSign(signID string)
}

// CoopPolicyProvider é a Port de saída que lê a política de co-op do jogo no
// game-registry (spec 007). Irmã do InvasionPolicyProvider — mesma origem, campos
// distintos. ErrProfileUnavailable em falha de leitura.
type CoopPolicyProvider interface {
	CoopPolicy(ctx context.Context, gameID string) (CoopPolicy, error)
}
