// Adapter Agones da Port domain.ServerAllocator (spec 004, ADR-0011/0012):
// Allocate cria uma GameServerAllocation via k8s API e Release deleta o
// GameServer (a Fleet repõe). Dynamic client de propósito — depender do módulo
// agones.dev/agones inteiro só pelos types não compensa.
package allocator

import (
	"context"
	"fmt"
	"net"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/session-service/internal/domain"
)

var (
	_ domain.ServerAllocator = (*Agones)(nil)
	_ domain.ServerRegistry  = (*Agones)(nil)
	_ domain.FleetInspector  = (*Agones)(nil)
)

var (
	allocationGVR = schema.GroupVersionResource{Group: "allocation.agones.dev", Version: "v1", Resource: "gameserverallocations"}
	gameServerGVR = schema.GroupVersionResource{Group: "agones.dev", Version: "v1", Resource: "gameservers"}
	fleetGVR      = schema.GroupVersionResource{Group: "agones.dev", Version: "v1", Resource: "fleets"}
)

// Agones aloca GameServers de uma Fleet via GameServerAllocation.
type Agones struct {
	client    dynamic.Interface
	namespace string
}

// NewAgones cria o adapter. A Fleet não é mais fixa na construção (ADR-0026) —
// cada chamada de Allocate resolve a Fleet do jogo (via ProfileSnapshot).
func NewAgones(client dynamic.Interface, namespace string) *Agones {
	return &Agones{client: client, namespace: namespace}
}

// Allocate cria uma GameServerAllocation com os metadados da partida. As
// annotations aterrissam SEM prefixo no GameServer (validado empiricamente,
// Agones 1.58) — o Unity lê `objectMeta.annotations["match_id"]` cru e rearma
// o gate. Fleet sem Ready → o Agones responde `UnAllocated` → ErrNoServer.
// fleet vazio → ErrFleetUnavailable, sem chamar a API k8s (ADR-0026): melhor
// falhar cedo e claro do que arriscar alocar da Fleet errada.
func (a *Agones) Allocate(ctx context.Context, matchID, gameID, fleet string) (domain.Server, error) {
	if fleet == "" {
		return domain.Server{}, fmt.Errorf("agones: fleet not set: %w", domain.ErrFleetUnavailable)
	}
	gsa := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "allocation.agones.dev/v1",
		"kind":       "GameServerAllocation",
		"metadata":   map[string]any{"generateName": "session-"},
		"spec": map[string]any{
			"selectors": []any{
				map[string]any{"matchLabels": map[string]any{"agones.dev/fleet": fleet}},
			},
			"metadata": map[string]any{
				"annotations": map[string]any{"match_id": matchID, "game_id": gameID},
			},
		},
	}}

	out, err := a.client.Resource(allocationGVR).Namespace(a.namespace).Create(ctx, gsa, metav1.CreateOptions{})
	if err != nil {
		return domain.Server{}, fmt.Errorf("agones: create allocation: %w", err)
	}

	state, _, _ := unstructured.NestedString(out.Object, "status", "state")
	if state != "Allocated" {
		return domain.Server{}, fmt.Errorf("agones: allocation %q: %w", state, domain.ErrNoServer)
	}

	name, _, _ := unstructured.NestedString(out.Object, "status", "gameServerName")
	address, _, _ := unstructured.NestedString(out.Object, "status", "address")
	ports, _, _ := unstructured.NestedSlice(out.Object, "status", "ports")
	if name == "" || address == "" || len(ports) == 0 {
		return domain.Server{}, fmt.Errorf("agones: allocation Allocated sem endereço/porta (gs=%q addr=%q)", name, address)
	}
	portMap, ok := ports[0].(map[string]any)
	if !ok {
		return domain.Server{}, fmt.Errorf("agones: status.ports[0] com shape inesperado: %T", ports[0])
	}
	port, _, _ := unstructured.NestedInt64(portMap, "port")

	return domain.Server{ID: name, Address: fmt.Sprintf("%s:%d", address, port)}, nil
}

// Release deleta o GameServer por nome; a Fleet repõe um Ready novo. NotFound
// não é erro: se o Unity já chamou Shutdown(), o objeto pode nem existir mais.
func (a *Agones) Release(ctx context.Context, serverID string) error {
	err := a.client.Resource(gameServerGVR).Namespace(a.namespace).Delete(ctx, serverID, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("agones: delete gameserver %q: %w", serverID, err)
	}
	return nil
}

// LiveServerIDs lista os GameServers vivos do namespace (sem filtro de Fleet —
// desde o ADR-0026 múltiplas Fleets, uma por jogo, coexistem no mesmo
// namespace; filtrar por uma só deixaria de reconciliar as sessões dos demais
// jogos) e devolve o conjunto dos `metadata.name` — o mesmo ID que Allocate
// grava em Server.ID (spec 008, ADR-0022). Uma chamada por tick do sweeper
// (barata; não N GETs).
func (a *Agones) LiveServerIDs(ctx context.Context) (map[string]struct{}, error) {
	list, err := a.client.Resource(gameServerGVR).Namespace(a.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("agones: list gameservers: %w", err)
	}
	live := make(map[string]struct{}, len(list.Items))
	for i := range list.Items {
		live[list.Items[i].GetName()] = struct{}{}
	}
	return live, nil
}

// FleetStatus lê o recurso Fleet pelo nome (spec 023). NotFound não é erro —
// FleetExists:false, sem os números. O campo Cluster do domain.FleetStatus
// fica vazio aqui de propósito: quem sabe o rótulo do cluster (AGONES_CLUSTER_LABEL)
// é o transport, não o adapter.
func (a *Agones) FleetStatus(ctx context.Context, fleetName string) (domain.FleetStatus, error) {
	out, err := a.client.Resource(fleetGVR).Namespace(a.namespace).Get(ctx, fleetName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return domain.FleetStatus{FleetExists: false}, nil
	}
	if err != nil {
		return domain.FleetStatus{}, fmt.Errorf("agones: get fleet %q: %w", fleetName, err)
	}

	desired, _, _ := unstructured.NestedInt64(out.Object, "spec", "replicas")
	current, _, _ := unstructured.NestedInt64(out.Object, "status", "replicas")
	ready, _, _ := unstructured.NestedInt64(out.Object, "status", "readyReplicas")
	allocated, _, _ := unstructured.NestedInt64(out.Object, "status", "allocatedReplicas")

	return domain.FleetStatus{
		FleetExists: true,
		Desired:     int(desired),
		Current:     int(current),
		Ready:       int(ready),
		Allocated:   int(allocated),
	}, nil
}

var _ domain.ServerAllocator = (*publicHost)(nil)

// publicHost decora um allocator reescrevendo o host do endereço devolvido,
// mantendo a porta dinâmica (DEV_PUBLIC_HOST, spec 004): o endereço do Agones
// é o IP interno do node k3d, inalcançável do Mac; a porta está mapeada.
type publicHost struct {
	inner domain.ServerAllocator
	host  string
}

// WithPublicHost devolve o allocator decorado, ou o original se host == "".
func WithPublicHost(inner domain.ServerAllocator, host string) domain.ServerAllocator {
	if host == "" {
		return inner
	}
	return &publicHost{inner: inner, host: host}
}

func (p *publicHost) Allocate(ctx context.Context, matchID, gameID, fleet string) (domain.Server, error) {
	server, err := p.inner.Allocate(ctx, matchID, gameID, fleet)
	if err != nil {
		return server, err
	}
	_, port, splitErr := net.SplitHostPort(server.Address)
	if splitErr != nil {
		return server, fmt.Errorf("agones: DEV_PUBLIC_HOST: endereço %q sem porta: %w", server.Address, splitErr)
	}
	server.Address = net.JoinHostPort(p.host, port)
	return server, nil
}

func (p *publicHost) Release(ctx context.Context, serverID string) error {
	return p.inner.Release(ctx, serverID)
}
