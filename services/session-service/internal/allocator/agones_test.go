package allocator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

// newFakeDynamic cria o fake do dynamic client com um reactor que responde ao
// create de GameServerAllocation com o status informado — o fake não tem o
// controller do Agones, então o status é simulado (spec 004: testes sem cluster).
func newFakeDynamic(t *testing.T, status map[string]any) *dynamicfake.FakeDynamicClient {
	t.Helper()
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "agones.dev", Version: "v1", Kind: "GameServerList"}, &unstructured.UnstructuredList{})
	client := dynamicfake.NewSimpleDynamicClient(scheme)
	client.PrependReactor("create", "gameserverallocations", func(action k8stesting.Action) (bool, runtime.Object, error) {
		obj := action.(k8stesting.CreateAction).GetObject().(*unstructured.Unstructured).DeepCopy()
		require.NoError(t, unstructured.SetNestedMap(obj.Object, status, "status"))
		return true, obj, nil
	})
	return client
}

func TestAgonesAllocate_Allocated_DevolveServerComEndereco(t *testing.T) {
	client := newFakeDynamic(t, map[string]any{
		"state":          "Allocated",
		"gameServerName": "packet-arena-abcde-12345",
		"address":        "172.19.0.3",
		"ports":          []any{map[string]any{"name": "default", "port": int64(7127)}},
	})

	agones := NewAgones(client, "default")
	server, err := agones.Allocate(context.Background(), "match-1", "game-1", "packet-arena")

	require.NoError(t, err)
	assert.Equal(t, "packet-arena-abcde-12345", server.ID)
	assert.Equal(t, "172.19.0.3:7127", server.Address)

	// a alocação leva os metadados da partida — annotations SEM prefixo
	// (validado empiricamente; é o que o Unity lê para rearmar o gate).
	actions := client.Actions()
	require.NotEmpty(t, actions)
	created := actions[len(actions)-1].(k8stesting.CreateAction).GetObject().(*unstructured.Unstructured)
	annotations, _, err := unstructured.NestedStringMap(created.Object, "spec", "metadata", "annotations")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"match_id": "match-1", "game_id": "game-1"}, annotations)
	selectors, _, err := unstructured.NestedSlice(created.Object, "spec", "selectors")
	require.NoError(t, err)
	require.Len(t, selectors, 1)
	labels, _, _ := unstructured.NestedStringMap(selectors[0].(map[string]any), "matchLabels")
	assert.Equal(t, map[string]string{"agones.dev/fleet": "packet-arena"}, labels)
}

// TestAgonesAllocate_FleetsDiferentes_SelectorUsaOValorPorChamada (ADR-0026):
// a Fleet não é mais fixa no adapter — cada chamada resolve a sua.
func TestAgonesAllocate_FleetsDiferentes_SelectorUsaOValorPorChamada(t *testing.T) {
	client := newFakeDynamic(t, map[string]any{
		"state": "Allocated", "gameServerName": "gs-1", "address": "172.19.0.3",
		"ports": []any{map[string]any{"name": "default", "port": int64(7127)}},
	})
	agones := NewAgones(client, "default")

	_, err := agones.Allocate(context.Background(), "match-1", "game-1", "packet-arena")
	require.NoError(t, err)
	_, err = agones.Allocate(context.Background(), "match-2", "game-2", "capoeira-souls")
	require.NoError(t, err)

	actions := client.Actions()
	require.Len(t, actions, 2)
	fleetOf := func(i int) string {
		created := actions[i].(k8stesting.CreateAction).GetObject().(*unstructured.Unstructured)
		selectors, _, _ := unstructured.NestedSlice(created.Object, "spec", "selectors")
		labels, _, _ := unstructured.NestedStringMap(selectors[0].(map[string]any), "matchLabels")
		return labels["agones.dev/fleet"]
	}
	assert.Equal(t, "packet-arena", fleetOf(0))
	assert.Equal(t, "capoeira-souls", fleetOf(1))
}

// TestAgonesAllocate_FleetVazia_ErrFleetUnavailable (ADR-0026): falha cedo, sem
// chamar a API k8s — melhor do que arriscar alocar da Fleet errada.
func TestAgonesAllocate_FleetVazia_ErrFleetUnavailable(t *testing.T) {
	client := newFakeDynamic(t, map[string]any{"state": "Allocated"})
	agones := NewAgones(client, "default")

	_, err := agones.Allocate(context.Background(), "match-1", "game-1", "")

	assert.ErrorIs(t, err, domain.ErrFleetUnavailable)
	assert.Empty(t, client.Actions(), "não chama a API k8s com fleet vazia")
}

func TestAgonesAllocate_UnAllocated_ErrNoServer(t *testing.T) {
	client := newFakeDynamic(t, map[string]any{"state": "UnAllocated"})

	agones := NewAgones(client, "default")
	_, err := agones.Allocate(context.Background(), "match-1", "game-1", "packet-arena")

	assert.ErrorIs(t, err, domain.ErrNoServer)
}

func TestAgonesRelease_GameServerInexistente_Idempotente(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "agones.dev", Version: "v1", Kind: "GameServerList"}, &unstructured.UnstructuredList{})
	client := dynamicfake.NewSimpleDynamicClient(scheme)
	client.PrependReactor("delete", "gameservers", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewNotFound(schema.GroupResource{Group: "agones.dev", Resource: "gameservers"}, "gs-sumiu")
	})

	agones := NewAgones(client, "default")
	// Unity já fez Shutdown() → o GameServer não existe mais; não é erro.
	assert.NoError(t, agones.Release(context.Background(), "gs-sumiu"))
}

func TestAgonesRelease_DeletaGameServerPorNome(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "agones.dev", Version: "v1", Kind: "GameServerList"}, &unstructured.UnstructuredList{})
	client := dynamicfake.NewSimpleDynamicClient(scheme)
	var deleted string
	client.PrependReactor("delete", "gameservers", func(action k8stesting.Action) (bool, runtime.Object, error) {
		deleted = action.(k8stesting.DeleteAction).GetName()
		return true, nil, nil
	})

	agones := NewAgones(client, "default")
	require.NoError(t, agones.Release(context.Background(), "packet-arena-abcde-12345"))
	assert.Equal(t, "packet-arena-abcde-12345", deleted)
}

// gameServerObj monta um GameServer unstructured com o label da fleet, para
// semear o fake do dynamic client no teste de LiveServerIDs.
func gameServerObj(name, fleet string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "agones.dev/v1",
		"kind":       "GameServer",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "default",
			"labels":    map[string]any{"agones.dev/fleet": fleet},
		},
	}}
}

// TestAgonesLiveServerIDs_DevolveTodasAsFleetsDoNamespace (ADR-0026): sem
// filtro de label — múltiplas Fleets (uma por jogo) coexistem no namespace, e o
// sweeper de reconciliação (ADR-0022) precisa enxergar todas, não só uma.
func TestAgonesLiveServerIDs_DevolveTodasAsFleetsDoNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "agones.dev", Version: "v1", Kind: "GameServerList"}, &unstructured.UnstructuredList{})
	client := dynamicfake.NewSimpleDynamicClient(scheme,
		gameServerObj("packet-arena-aaaaa", "packet-arena"),
		gameServerObj("packet-arena-bbbbb", "packet-arena"),
		gameServerObj("capoeira-souls-ccccc", "capoeira-souls"),
	)

	agones := NewAgones(client, "default")
	live, err := agones.LiveServerIDs(context.Background())

	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{
		"packet-arena-aaaaa":  {},
		"packet-arena-bbbbb":  {},
		"capoeira-souls-ccccc": {},
	}, live, "todos os GameServers do namespace, de qualquer fleet")
}

func TestWithPublicHost_ReescreveHostMantemPorta(t *testing.T) {
	inner := New([]string{"172.19.0.3:7127"})
	alloc := WithPublicHost(inner, "127.0.0.1")

	server, err := alloc.Allocate(context.Background(), "match-1", "game-1", "")

	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:7127", server.Address, "host reescrito, porta dinâmica mantida")
}

func TestWithPublicHost_HostVazio_NaoDecoraNada(t *testing.T) {
	inner := New([]string{"172.19.0.3:7127"})
	assert.Same(t, any(inner), any(WithPublicHost(inner, "")), "sem DEV_PUBLIC_HOST devolve o allocator cru")
}
