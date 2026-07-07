package domain

import "errors"

// ErrNoServer indica que o session-service não tem game server livre para a
// sessão (mapeado de codes.ResourceExhausted pelo adapter do session).
var ErrNoServer = errors.New("matchmaking: no server available")

// ErrProfileUnavailable indica que o tamanho do match não pôde ser derivado do
// perfil de multiplayer do jogo (ADR-0023): jogo sem perfil, arquétipo !=
// queue_arena, ou game-registry indisponível. O ticket não entra na fila.
var ErrProfileUnavailable = errors.New("matchmaking: multiplayer profile unavailable")

// ErrInvasionNotSupported indica que o jogo não aceita invasão (host_world_dropin,
// spec 006): arquétipo diferente ou InvasionEnabled=false. Mapeado para
// FailedPrecondition no transport.
var ErrInvasionNotSupported = errors.New("matchmaking: invasion not supported")

// ErrNoEligibleWorld indica que nenhum mundo aberto casou com o invasor (spec
// 006): lista vazia, nenhum na faixa, ou todos cheios na corrida. Mapeado para
// NotFound — o invasor tenta de novo mais tarde (não há fila de espera).
var ErrNoEligibleWorld = errors.New("matchmaking: no eligible world")

// ErrRoleFull / ErrWorldNotFound são erros internos do injetor (WorldInjector),
// traduzidos do gRPC do session. ErrRoleFull dirige o retry (mundo encheu na
// corrida listar→injetar); ErrWorldNotFound é o mundo que fechou entre listar e
// injetar. Nenhum dos dois vaza para o cliente do Invade.
var (
	ErrRoleFull     = errors.New("matchmaking: invader slot full")
	ErrWorldNotFound = errors.New("matchmaking: world not found")
)

// ErrCoopNotSupported indica que o jogo não aceita co-op (host_world_dropin,
// spec 007): arquétipo diferente ou papel coop não habilitado. FailedPrecondition.
var ErrCoopNotSupported = errors.New("matchmaking: coop not supported")

// ErrSignNotFound / ErrSignConsumed são os erros da invocação de um sinal (spec
// 007): o sinal não existe/expirou (NotFound), ou já foi consumido por outra
// invocação (FailedPrecondition — idempotência: consumo no máximo uma vez).
var (
	ErrSignNotFound = errors.New("matchmaking: summon sign not found")
	ErrSignConsumed = errors.New("matchmaking: summon sign already consumed")
)
