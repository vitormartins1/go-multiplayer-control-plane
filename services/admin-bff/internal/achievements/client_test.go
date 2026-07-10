package achievements

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/admin-bff/internal/domain"
)

func TestCatalogForGame_Mapeia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/games/g1/achievements", r.URL.Path)
		_, _ = w.Write([]byte(`{"achievements":[{"id":"first_win","name":"Primeira","description":"x","target":1}]}`))
	}))
	defer srv.Close()

	cat, err := New(srv.URL, srv.Client()).CatalogForGame(context.Background(), "g1")
	require.NoError(t, err)
	require.Len(t, cat, 1)
	assert.Equal(t, "first_win", cat[0].ID)
}

func TestCatalogForGame_MapeiaStatCriterio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"achievements":[{"id":"ace","name":"Ás","description":"x","statKey":"kills","statMode":"per_match","target":5}]}`))
	}))
	defer srv.Close()

	cat, err := New(srv.URL, srv.Client()).CatalogForGame(context.Background(), "g1")
	require.NoError(t, err)
	require.Len(t, cat, 1)
	assert.Equal(t, "kills", cat[0].StatKey)
	assert.Equal(t, "per_match", cat[0].StatMode)
	assert.Empty(t, cat[0].EventType)
}

func TestCreateAchievement_EnviaStatCriterio(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"ace","name":"Ás","description":"x","statKey":"kills","statMode":"per_match","target":5}`))
	}))
	defer srv.Close()

	err := New(srv.URL, srv.Client()).CreateAchievement(context.Background(), "g1", domain.Achievement{
		ID: "ace", Name: "Ás", StatKey: "kills", StatMode: "per_match", Target: 5,
	})
	require.NoError(t, err)
	assert.Contains(t, gotBody, `"statKey":"kills"`)
	assert.Contains(t, gotBody, `"statMode":"per_match"`)
}

func TestPlayerAchievements_Mapeia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/games/g1/players/p1/achievements", r.URL.Path)
		_, _ = w.Write([]byte(`{"achievements":[{"id":"first_win","name":"P","description":"x","progress":1,"target":1,"unlocked":true,"unlocked_at":"2026-06-29T10:00:00Z"}]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).PlayerAchievements(context.Background(), "g1", "p1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.True(t, got[0].Unlocked)
	require.NotNil(t, got[0].UnlockedAt)
	assert.False(t, got[0].UnlockedAt.IsZero())
}

// Conquista em progresso (não desbloqueada) não deve trazer UnlockedAt nenhum
// — nem o zero-time do Go (0001-01-01). Regressão do nit registrado em
// memória: sem o ponteiro, `omitempty` não omite struct time.Time e o zero
// vazava pro front como data "desbloqueada em 0001".
func TestPlayerAchievements_EmProgresso_UnlockedAtNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"achievements":[{"id":"first_win","name":"P","description":"x","progress":0,"target":1,"unlocked":false}]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL, srv.Client()).PlayerAchievements(context.Background(), "g1", "p1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.False(t, got[0].Unlocked)
	assert.Nil(t, got[0].UnlockedAt)
}

func TestCreateAchievement_201(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/games/g1/achievements", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"speedrun","name":"Speed","description":"x","target":1}`))
	}))
	defer srv.Close()

	err := New(srv.URL, srv.Client()).CreateAchievement(context.Background(), "g1", domain.Achievement{
		ID: "speedrun", Name: "Speed", EventType: "fast_win", Target: 1,
	})
	require.NoError(t, err)
}

func TestCreateAchievement_409_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"achievement_exists"}`))
	}))
	defer srv.Close()

	err := New(srv.URL, srv.Client()).CreateAchievement(context.Background(), "g1", domain.Achievement{ID: "x", Name: "x", EventType: "kill", Target: 1})
	var ue *domain.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusConflict, ue.Status)
}

func TestCatalogForGame_Fora_Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	_, err := New(url, &http.Client{}).CatalogForGame(context.Background(), "g1")
	assert.ErrorIs(t, err, domain.ErrAchievementsUnavailable)
}

func TestUpdateAchievement_PUT_200(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := New(srv.URL, srv.Client()).UpdateAchievement(context.Background(), "g1", "first_win",
		domain.Achievement{Name: "N", EventType: "e", Target: 2})
	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, gotMethod)
	assert.Equal(t, "/admin/games/g1/achievements/first_win", gotPath)
}

func TestUpdateAchievement_404_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"achievement_not_found"}`))
	}))
	defer srv.Close()

	err := New(srv.URL, srv.Client()).UpdateAchievement(context.Background(), "g1", "x", domain.Achievement{Name: "N", EventType: "e", Target: 1})
	var ue *domain.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusNotFound, ue.Status)
}

func TestDeleteAchievement_DELETE_204(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := New(srv.URL, srv.Client()).DeleteAchievement(context.Background(), "g1", "first_win")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/admin/games/g1/achievements/first_win", gotPath)
}

func TestImportAchievements_TodosCriados(t *testing.T) {
	var reqs int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs++
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/admin/games/g1/achievements", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"x","name":"N","description":"","target":1}`))
	}))
	defer srv.Close()

	items := []domain.Achievement{
		{ID: "a1", Name: "A1", EventType: "e", Target: 1},
		{ID: "a2", Name: "A2", EventType: "e", Target: 2},
	}
	resp, err := New(srv.URL, srv.Client()).ImportAchievements(context.Background(), "g1", items)
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Created)
	assert.Equal(t, 0, resp.Failed)
	require.Len(t, resp.Results, 2)
	assert.Equal(t, domain.ImportStatusCreated, resp.Results[0].Status)
	assert.Equal(t, "a1", resp.Results[0].ID)
	assert.Equal(t, 1, resp.Results[0].Row)
	assert.Equal(t, domain.ImportStatusCreated, resp.Results[1].Status)
	assert.Equal(t, 2, reqs)
}

func TestImportAchievements_DuplicataParcial(t *testing.T) {
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call++
		switch call {
		case 1:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"a1","name":"A1","target":1}`))
		case 2:
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"achievement_exists"}`))
		case 3:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_achievement"}`))
		}
	}))
	defer srv.Close()

	items := []domain.Achievement{
		{ID: "a1", Name: "A1", EventType: "e", Target: 1},
		{ID: "a2", Name: "A2", EventType: "e", Target: 1},
		{ID: "a3", Name: "A3", EventType: "e", Target: 1},
	}
	resp, err := New(srv.URL, srv.Client()).ImportAchievements(context.Background(), "g1", items)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Created)
	assert.Equal(t, 2, resp.Failed)
	assert.Equal(t, domain.ImportStatusCreated, resp.Results[0].Status)
	assert.Equal(t, domain.ImportStatusDuplicate, resp.Results[1].Status)
	assert.Equal(t, "achievement_exists", resp.Results[1].Error)
	assert.Equal(t, domain.ImportStatusInvalid, resp.Results[2].Status)
	assert.Equal(t, "invalid_achievement", resp.Results[2].Error)
	assert.Equal(t, 3, resp.Results[2].Row)
}

func TestImportAchievements_ServicoFora_StatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	items := []domain.Achievement{{ID: "a1", Name: "A1", EventType: "e", Target: 1}}
	resp, err := New(url, &http.Client{}).ImportAchievements(context.Background(), "g1", items)
	require.NoError(t, err) // nunca retorna erro não-nil — absorvido por linha
	assert.Equal(t, 0, resp.Created)
	assert.Equal(t, 1, resp.Failed)
	assert.Equal(t, domain.ImportStatusError, resp.Results[0].Status)
	assert.Equal(t, "service_unavailable", resp.Results[0].Error)
}

func TestImportAchievements_OrdemPreservada(t *testing.T) {
	var received []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		received = append(received, body.ID)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	items := []domain.Achievement{
		{ID: "z", Name: "Z", EventType: "e", Target: 1},
		{ID: "a", Name: "A", EventType: "e", Target: 1},
		{ID: "m", Name: "M", EventType: "e", Target: 1},
	}
	resp, err := New(srv.URL, srv.Client()).ImportAchievements(context.Background(), "g1", items)
	require.NoError(t, err)
	assert.Equal(t, []string{"z", "a", "m"}, received)
	assert.Equal(t, 1, resp.Results[0].Row)
	assert.Equal(t, 2, resp.Results[1].Row)
	assert.Equal(t, 3, resp.Results[2].Row)
}

func TestGameEventRules_Mapeia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/games/g1/event-rules", r.URL.Path)
		_, _ = w.Write([]byte(`{"rules":[{"id":"r1","scope":"global","archetype":"host_world_dropin","match":{"event":"JOINED"},"eventTypes":["world_joined"]}]}`))
	}))
	defer srv.Close()

	rules, err := New(srv.URL, srv.Client()).GameEventRules(context.Background(), "g1")
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, "global", rules[0].Scope)
	assert.Equal(t, []string{"world_joined"}, rules[0].EventTypes)
}

func TestCreateEventRule_201(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"r2","scope":"game","archetype":"battle_royale","match":{"placement":"1"},"eventTypes":["custom_win_badge"]}`))
	}))
	defer srv.Close()

	created, err := New(srv.URL, srv.Client()).CreateEventRule(context.Background(), "g1", domain.EventRule{
		Archetype: "battle_royale", Match: map[string]json.RawMessage{"placement": json.RawMessage(`"1"`)}, EventTypes: []string{"custom_win_badge"},
	})
	require.NoError(t, err)
	assert.Contains(t, gotBody, `"archetype":"battle_royale"`)
	assert.Equal(t, "game", created.Scope)
	assert.Equal(t, "r2", created.ID)
}

func TestCreateEventRule_400_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_event_rule"}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL, srv.Client()).CreateEventRule(context.Background(), "g1", domain.EventRule{Archetype: "x"})
	var ue *domain.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusBadRequest, ue.Status)
}

func TestDeleteEventRule_DELETE_204(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := New(srv.URL, srv.Client()).DeleteEventRule(context.Background(), "g1", "r1")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, gotMethod)
	assert.Equal(t, "/admin/games/g1/event-rules/r1", gotPath)
}

func TestDeleteEventRule_404_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"event_rule_not_found"}`))
	}))
	defer srv.Close()

	err := New(srv.URL, srv.Client()).DeleteEventRule(context.Background(), "g1", "global-1")
	var ue *domain.UpstreamError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusNotFound, ue.Status, "global ou inexistente: mesma resposta")
}
