//go:build integration

// Testes de integração com Postgres real (testcontainers). Rodar com:
//
//	go test -tags=integration ./services/game-registry/internal/repository/
//
// Requer Docker em execução.
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/game-registry/internal/domain"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("game_registry"),
		tcpostgres.WithUsername("dev"),
		tcpostgres.WithPassword("dev"),
		tcpostgres.WithInitScripts(
			"../../migrations/001_create_games.sql",
			"../../migrations/004_add_cover_metadata.sql",
			"../../migrations/005_add_multiplayer_profile.sql",
		),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func newGame(slug, name string) *domain.Game {
	return &domain.Game{ID: uuid.NewString(), Slug: slug, Name: name, CreatedAt: time.Now().UTC()}
}

func TestPostgresGameRepository_CreateEFindByID(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()

	g := newGame("tetris", "Tetris")
	require.NoError(t, repo.Create(ctx, g))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	assert.Equal(t, g.ID, got.ID)
	assert.Equal(t, "tetris", got.Slug)
	assert.Equal(t, "Tetris", got.Name)
}

func TestPostgresGameRepository_SlugDuplicadoCaseInsensitive_ErrSlugTaken(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, newGame("tetris", "Tetris")))
	err := repo.Create(ctx, newGame("TETRIS", "Outro Tetris")) // mesmo slug, case diferente
	assert.ErrorIs(t, err, domain.ErrSlugTaken)
}

func TestPostgresGameRepository_List_OrdenadoPorNome(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newGame("zelda", "Zelda")))
	require.NoError(t, repo.Create(ctx, newGame("arkanoid", "Arkanoid")))
	require.NoError(t, repo.Create(ctx, newGame("mario", "Mario")))

	got, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []string{"Arkanoid", "Mario", "Zelda"}, []string{got[0].Name, got[1].Name, got[2].Name})
}

func TestPostgresGameRepository_FindByID_Inexistente_ErrGameNotFound(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	_, err := repo.FindByID(context.Background(), "nao-existe")
	assert.ErrorIs(t, err, domain.ErrGameNotFound)
}

func TestPostgresGameRepository_CapaRoundTrip(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()

	g := newGame("tetris", "Tetris")
	g.ImageSource = "generated"
	g.GeneratedImageSeed = "tetris"
	g.GeneratedImageStyle = "shapes"
	require.NoError(t, repo.Create(ctx, g))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	assert.Equal(t, "generated", got.ImageSource)
	assert.Equal(t, "tetris", got.GeneratedImageSeed)
	assert.Equal(t, "shapes", got.GeneratedImageStyle)
	assert.Empty(t, got.Image, "sem upload -> image vazio (NULLIF gravou NULL, COALESCE leu '')")
}

func TestPostgresGameRepository_RegistroAntigoSemSeed_CaiNoSlug(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPostgresGameRepository(pool)
	ctx := context.Background()

	// simula um registro "antigo": insere direto com seed/image NULL.
	_, err := pool.Exec(ctx,
		`INSERT INTO games (id, slug, name, created_at, image, generated_image_seed)
		 VALUES ($1, 'legado', 'Jogo Legado', now(), NULL, NULL)`, "old-1")
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, "old-1")
	require.NoError(t, err)
	assert.Equal(t, "legado", got.GeneratedImageSeed, "seed NULL -> slug (COALESCE)")
	assert.Equal(t, "generated", got.ImageSource, "default da migration")
	assert.Equal(t, "shapes", got.GeneratedImageStyle, "default da migration")
	assert.Empty(t, got.Image)
}

func TestPostgresGameRepository_UpdateName_AtualizaSoNome(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()
	g := newGame("orb", "Orb Rush")
	require.NoError(t, repo.Create(ctx, g))

	require.NoError(t, repo.UpdateName(ctx, g.ID, "Orb Rush 2"))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	assert.Equal(t, "Orb Rush 2", got.Name)
	assert.Equal(t, "orb", got.Slug, "slug intacto")
}

func TestPostgresGameRepository_UpdateName_Inexistente_ErrGameNotFound(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	err := repo.UpdateName(context.Background(), "00000000-0000-0000-0000-000000000000", "X")
	assert.ErrorIs(t, err, domain.ErrGameNotFound)
}

func teamsProfile() domain.MultiplayerProfile {
	return domain.MultiplayerProfile{
		Archetype: domain.ArchetypeQueueArena, Topology: domain.TopologyDedicated,
		QueueArena: &domain.QueueArenaParams{
			Structure: "teams", TeamCount: 2, TeamSize: 3,
			WinCondition: "first_to_score", ScoreTarget: 50,
			MatchStructure: "single", AllowDraw: true,
			ScoringStats: []string{"kills", "assists"},
		},
	}
}

func TestPostgresGameRepository_SetModes_RoundTrip(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()
	g := newGame("arena", "Arena")
	require.NoError(t, repo.Create(ctx, g))

	p := teamsProfile()
	require.NoError(t, repo.SetModes(ctx, g.ID, []domain.MultiplayerMode{{ModeID: domain.DefaultModeID, Profile: p}}))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	require.NotNil(t, got.MultiplayerProfile)
	assert.Equal(t, domain.ArchetypeQueueArena, got.MultiplayerProfile.Archetype)
	assert.Equal(t, domain.TopologyDedicated, got.MultiplayerProfile.Topology)
	require.NotNil(t, got.MultiplayerProfile.QueueArena)
	assert.Equal(t, *p.QueueArena, *got.MultiplayerProfile.QueueArena)
}

func TestPostgresGameRepository_SetModes_Substitui(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()
	g := newGame("arena", "Arena")
	require.NoError(t, repo.Create(ctx, g))
	require.NoError(t, repo.SetModes(ctx, g.ID, []domain.MultiplayerMode{{ModeID: domain.DefaultModeID, Profile: teamsProfile()}}))

	ffa := domain.MultiplayerProfile{
		Archetype: domain.ArchetypeQueueArena, Topology: domain.TopologyDedicated,
		QueueArena: &domain.QueueArenaParams{
			Structure: "ffa", PlayerCount: 8,
			WinCondition: "highest_at_time", DurationSeconds: 300, MatchStructure: "single",
		},
	}
	require.NoError(t, repo.SetModes(ctx, g.ID, []domain.MultiplayerMode{{ModeID: domain.DefaultModeID, Profile: ffa}}))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	require.NotNil(t, got.MultiplayerProfile.QueueArena)
	assert.Equal(t, "ffa", got.MultiplayerProfile.QueueArena.Structure)
	assert.Equal(t, 8, got.MultiplayerProfile.QueueArena.PlayerCount)
	assert.Zero(t, got.MultiplayerProfile.QueueArena.TeamCount, "params antigos substituídos, não mesclados")
}

func TestPostgresGameRepository_SetModes_BattleRoyale_RoundTrip(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()
	g := newGame("br", "Battle Royale")
	require.NoError(t, repo.Create(ctx, g))

	p := domain.MultiplayerProfile{
		Archetype: domain.ArchetypeBattleRoyale, Topology: domain.TopologyDedicated,
		BattleRoyale: &domain.BattleRoyaleParams{
			LobbySize: 60, TeamSize: 4, RespawnMode: domain.RespawnArena1v1,
			ScoringStats: []string{"kills"},
		},
	}
	require.NoError(t, repo.SetModes(ctx, g.ID, []domain.MultiplayerMode{{ModeID: domain.DefaultModeID, Profile: p}}))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	require.NotNil(t, got.MultiplayerProfile)
	assert.Equal(t, domain.ArchetypeBattleRoyale, got.MultiplayerProfile.Archetype)
	require.NotNil(t, got.MultiplayerProfile.BattleRoyale)
	assert.Equal(t, *p.BattleRoyale, *got.MultiplayerProfile.BattleRoyale)
	assert.Nil(t, got.MultiplayerProfile.QueueArena, "só o ponteiro do arquétipo declarado é não-nil")
}

func TestPostgresGameRepository_SetModes_TrocaArquetipo(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()
	g := newGame("mut", "Mut")
	require.NoError(t, repo.Create(ctx, g))
	require.NoError(t, repo.SetModes(ctx, g.ID, []domain.MultiplayerMode{{ModeID: domain.DefaultModeID, Profile: teamsProfile()}}))

	br := domain.MultiplayerProfile{
		Archetype: domain.ArchetypeBattleRoyale, Topology: domain.TopologyDedicated,
		BattleRoyale: &domain.BattleRoyaleParams{LobbySize: 100, TeamSize: 1, RespawnMode: domain.RespawnOneLife},
	}
	require.NoError(t, repo.SetModes(ctx, g.ID, []domain.MultiplayerMode{{ModeID: domain.DefaultModeID, Profile: br}}))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	require.NotNil(t, got.MultiplayerProfile.BattleRoyale)
	assert.Equal(t, 100, got.MultiplayerProfile.BattleRoyale.LobbySize)
	assert.Nil(t, got.MultiplayerProfile.QueueArena, "ponteiro do arquétipo antigo fica nil após a troca")
}

func TestPostgresGameRepository_SetModes_CoopMission_RoundTrip(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()
	g := newGame("cm", "Coop Mission")
	require.NoError(t, repo.Create(ctx, g))

	p := domain.MultiplayerProfile{
		Archetype: domain.ArchetypeCoopMission, Topology: domain.TopologyDedicated,
		CoopMission: &domain.CoopMissionParams{
			TeamSize: 4, Missions: []string{"salvage_op", "escort"},
			Difficulties: []string{"hazard_1", "hazard_5"}, BotfillEnabled: true,
			ScoringStats: []string{"kills", "revives"},
		},
	}
	require.NoError(t, repo.SetModes(ctx, g.ID, []domain.MultiplayerMode{{ModeID: domain.DefaultModeID, Profile: p}}))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	require.NotNil(t, got.MultiplayerProfile)
	assert.Equal(t, domain.ArchetypeCoopMission, got.MultiplayerProfile.Archetype)
	require.NotNil(t, got.MultiplayerProfile.CoopMission)
	assert.Equal(t, *p.CoopMission, *got.MultiplayerProfile.CoopMission)
	assert.Nil(t, got.MultiplayerProfile.BattleRoyale, "só o ponteiro do arquétipo declarado é não-nil")
}

func TestPostgresGameRepository_SetModes_TrocaParaCoopMission(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()
	g := newGame("mut2", "Mut2")
	require.NoError(t, repo.Create(ctx, g))
	br := domain.MultiplayerProfile{
		Archetype: domain.ArchetypeBattleRoyale, Topology: domain.TopologyDedicated,
		BattleRoyale: &domain.BattleRoyaleParams{LobbySize: 100, TeamSize: 1, RespawnMode: domain.RespawnOneLife},
	}
	require.NoError(t, repo.SetModes(ctx, g.ID, []domain.MultiplayerMode{{ModeID: domain.DefaultModeID, Profile: br}}))

	cm := domain.MultiplayerProfile{
		Archetype: domain.ArchetypeCoopMission, Topology: domain.TopologyDedicated,
		CoopMission: &domain.CoopMissionParams{TeamSize: 4, BotfillEnabled: true},
	}
	require.NoError(t, repo.SetModes(ctx, g.ID, []domain.MultiplayerMode{{ModeID: domain.DefaultModeID, Profile: cm}}))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	require.NotNil(t, got.MultiplayerProfile.CoopMission)
	assert.Equal(t, 4, got.MultiplayerProfile.CoopMission.TeamSize)
	assert.Nil(t, got.MultiplayerProfile.BattleRoyale, "ponteiro do arquétipo antigo fica nil após a troca")
}

func TestPostgresGameRepository_FindByID_SemPerfil_Nil(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()
	g := newGame("arena", "Arena")
	require.NoError(t, repo.Create(ctx, g))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	assert.Nil(t, got.MultiplayerProfile, "jogo recém-criado não tem perfil")
}

func TestPostgresGameRepository_SetModes_Inexistente_ErrGameNotFound(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	err := repo.SetModes(context.Background(), "00000000-0000-0000-0000-000000000000",
		[]domain.MultiplayerMode{{ModeID: domain.DefaultModeID, Profile: teamsProfile()}})
	assert.ErrorIs(t, err, domain.ErrGameNotFound)
}

// --- modos nomeados (ADR-0024, spec 009) ---

func TestPostgresGameRepository_SetModes_MultiModo_RoundTripOrdenado(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()
	g := newGame("multi", "Multi")
	require.NoError(t, repo.Create(ctx, g))

	warzone := domain.MultiplayerProfile{
		Archetype: domain.ArchetypeBattleRoyale, Topology: domain.TopologyDedicated,
		BattleRoyale: &domain.BattleRoyaleParams{LobbySize: 60, TeamSize: 3, RespawnMode: domain.RespawnRebootTimer},
	}
	require.NoError(t, repo.SetModes(ctx, g.ID, []domain.MultiplayerMode{
		{ModeID: "warzone", DisplayName: "Warzone", Profile: warzone},
		{ModeID: domain.DefaultModeID, DisplayName: "TDM", Profile: teamsProfile()},
	}))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, got.MultiplayerModes, 2)
	assert.Equal(t, domain.DefaultModeID, got.MultiplayerModes[0].ModeID, "default primeiro")
	assert.Equal(t, "TDM", got.MultiplayerModes[0].DisplayName)
	assert.Equal(t, "warzone", got.MultiplayerModes[1].ModeID)
	require.NotNil(t, got.MultiplayerModes[1].Profile.BattleRoyale)
	assert.Equal(t, 60, got.MultiplayerModes[1].Profile.BattleRoyale.LobbySize)
	require.NotNil(t, got.MultiplayerProfile, "compat: MultiplayerProfile = perfil do default")
	assert.Equal(t, domain.ArchetypeQueueArena, got.MultiplayerProfile.Archetype)
}

func TestPostgresGameRepository_DecodeModes_UpgradeLazyDeV1(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPostgresGameRepository(pool)
	ctx := context.Background()
	g := newGame("legado", "Legado")
	require.NoError(t, repo.Create(ctx, g))

	// Grava o formato v1 (objeto único no topo) direto na coluna, como um jogo
	// pré-ADR-0024 teria.
	v1 := `{"archetype":"queue_arena","topology":"dedicated","params":{"structure":"ffa","playerCount":4,"winCondition":"first_to_score","scoreTarget":5,"matchStructure":"single","allowDraw":false}}`
	_, err := pool.Exec(ctx, `UPDATE games SET multiplayer_profile = $2 WHERE id = $1`, g.ID, []byte(v1))
	require.NoError(t, err)

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, got.MultiplayerModes, 1, "v1 lido como o modo default")
	assert.Equal(t, domain.DefaultModeID, got.MultiplayerModes[0].ModeID)
	assert.Empty(t, got.MultiplayerModes[0].DisplayName)
	require.NotNil(t, got.MultiplayerProfile)
	require.NotNil(t, got.MultiplayerProfile.QueueArena)
	assert.Equal(t, 4, got.MultiplayerProfile.QueueArena.PlayerCount)

	// O próximo write persiste v2 (upgrade lazy): regrava e relê.
	require.NoError(t, repo.SetModes(ctx, g.ID, got.MultiplayerModes))
	var raw []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT multiplayer_profile FROM games WHERE id = $1`, g.ID).Scan(&raw))
	assert.Contains(t, string(raw), `"modes"`, "coluna reescrita no formato v2")
}

func TestPostgresGameRepository_SetModes_DeletePersiste(t *testing.T) {
	repo := NewPostgresGameRepository(newTestPool(t))
	ctx := context.Background()
	g := newGame("del", "Del")
	require.NoError(t, repo.Create(ctx, g))

	require.NoError(t, repo.SetModes(ctx, g.ID, []domain.MultiplayerMode{
		{ModeID: domain.DefaultModeID, Profile: teamsProfile()},
		{ModeID: "ranked", Profile: teamsProfile()},
	}))
	// Simula o DeleteMode do service: replace sem o modo removido.
	require.NoError(t, repo.SetModes(ctx, g.ID, []domain.MultiplayerMode{
		{ModeID: domain.DefaultModeID, Profile: teamsProfile()},
	}))

	got, err := repo.FindByID(ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, got.MultiplayerModes, 1)
	assert.Equal(t, domain.DefaultModeID, got.MultiplayerModes[0].ModeID)
}
