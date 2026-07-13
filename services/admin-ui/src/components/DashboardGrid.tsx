import { useEffect, useMemo, useState } from "react";
import { ActionIcon, Button, Group, Paper, Text } from "@mantine/core";
import { IconEyeOff, IconLayoutGrid, IconRestore } from "@tabler/icons-react";
import type { Icon } from "@tabler/icons-react";
import ReactGridLayout, { WidthProvider, type Layout } from "react-grid-layout/legacy";
import "react-grid-layout/css/styles.css";
import "react-resizable/css/styles.css";
import { defaultLayout, loadLayout, saveLayout, clearLayout, type WidgetLayoutItem } from "../lib/dashboardLayout";

const GridLayout = WidthProvider(ReactGridLayout);
const ROW_HEIGHT = 30;

export interface WidgetDef {
  id: string;
  title: string;
  defaultH: number; // em linhas do grid (rowHeight)
  // icon é só apresentação (badge de arquétipo, spec 064) — não entra no
  // layout persistido (WidgetLayoutItem/localStorage).
  icon?: Icon;
  render: () => React.ReactNode;
}

// DashboardGrid é a casca configurável da aba Analytics (spec 062):
// drag/resize/reorder/esconder widgets, persistido em localStorage por jogo.
// Cada widget é um componente React comum (os gráficos existentes não mudam,
// só ganham essa casca de layout).
export function DashboardGrid({ gameId, widgets }: { gameId: string; widgets: WidgetDef[] }) {
  const catalogKey = useMemo(() => widgets.map((w) => w.id).join(","), [widgets]);
  const catalogMeta = useMemo(() => widgets.map((w) => ({ id: w.id, defaultH: w.defaultH })), [catalogKey]); // eslint-disable-line react-hooks/exhaustive-deps

  const [state, setState] = useState(() => loadLayout(gameId, catalogMeta));
  const [editing, setEditing] = useState(false);

  // troca de jogo -> recarrega o layout DAQUELE jogo (não vaza do anterior).
  useEffect(() => {
    setState(loadLayout(gameId, catalogMeta));
    setEditing(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gameId, catalogKey]);

  function persist(next: typeof state) {
    setState(next);
    saveLayout(gameId, next);
  }

  function handleLayoutChange(layout: Layout) {
    if (!editing) return; // ignora o onLayoutChange inicial de montagem (fora do modo de edição, é read-only)
    const items: WidgetLayoutItem[] = layout.map((l) => ({ i: l.i, x: l.x, y: l.y, w: l.w, h: l.h }));
    persist({ ...state, items });
  }

  function hideWidget(id: string) {
    persist({ ...state, hidden: [...state.hidden, id] });
  }

  function restoreDefault() {
    clearLayout(gameId);
    const next = defaultLayout(catalogMeta);
    setState(next);
  }

  const byId = new Map(widgets.map((w) => [w.id, w]));
  const visibleItems = state.items.filter((it) => byId.has(it.i) && !state.hidden.includes(it.i));
  const layout: Layout = visibleItems.map((it) => ({ ...it, static: !editing }));

  return (
    <div>
      <Group justify="flex-end" mb="sm" gap="xs">
        {state.hidden.length > 0 && (
          <Text size="xs" c="dimmed">
            {state.hidden.length} widget(s) escondido(s)
          </Text>
        )}
        <Button
          variant={editing ? "filled" : "default"}
          size="xs"
          leftSection={<IconLayoutGrid size={14} />}
          onClick={() => setEditing((v) => !v)}
        >
          {editing ? "Concluir edição" : "Editar layout"}
        </Button>
        <Button variant="default" size="xs" leftSection={<IconRestore size={14} />} onClick={restoreDefault}>
          Restaurar padrão
        </Button>
      </Group>

      <GridLayout
        layout={layout}
        cols={12}
        rowHeight={ROW_HEIGHT}
        margin={[12, 12]}
        isDraggable={editing}
        isResizable={editing}
        onLayoutChange={handleLayoutChange}
        useCSSTransforms
      >
        {visibleItems.map((it) => {
          const widget = byId.get(it.i)!;
          return (
            <div key={it.i}>
              <Paper withBorder p="md" h="100%" style={{ overflow: "auto" }}>
                <Group justify="space-between" mb="xs" wrap="nowrap">
                  <Group gap={6} wrap="nowrap">
                    {widget.icon && <widget.icon size={14} />}
                    <Text fw={600} size="sm">
                      {widget.title}
                    </Text>
                  </Group>
                  {editing && (
                    <ActionIcon variant="subtle" color="gray" size="sm" onClick={() => hideWidget(it.i)}>
                      <IconEyeOff size={14} />
                    </ActionIcon>
                  )}
                </Group>
                {widget.render()}
              </Paper>
            </div>
          );
        })}
      </GridLayout>
    </div>
  );
}
