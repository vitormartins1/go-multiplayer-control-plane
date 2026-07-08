import { Badge, Box, Group, Stack, Text } from "@mantine/core";

// JsonMaterialized: leitura "consolidada" de um JSON (spec 042) — o mesmo
// conteúdo do tree, materializado para leitura rápida, sem sintaxe:
//   escalar            -> linha "chave  valor" (bool vira badge)
//   array de escalares -> linha "chave  [badges]"
//   objeto aninhado    -> sub-seção indentada com borda à esquerda
//   array de objetos   -> itens numerados (#1, #2…), cada um como sub-seção
// Read-only e sem dependências: composição recursiva de Mantine.

function isScalar(v: unknown): boolean {
  return v === null || typeof v !== "object";
}

function Leaf({ value }: { value: unknown }) {
  if (typeof value === "boolean") {
    return (
      <Badge size="sm" variant="light" color={value ? "green" : "gray"}>
        {String(value)}
      </Badge>
    );
  }
  if (value === null) {
    return (
      <Text span size="sm" c="dimmed" fs="italic">
        null
      </Text>
    );
  }
  if (typeof value === "number") {
    return (
      <Text span size="sm" fw={600} ff="monospace">
        {value.toLocaleString("pt-BR")}
      </Text>
    );
  }
  return (
    <Text span size="sm">
      {String(value)}
    </Text>
  );
}

function KeyLabel({ name }: { name: string }) {
  return (
    <Text span size="xs" c="dimmed" tt="uppercase" fw={600} style={{ letterSpacing: 0.4 }}>
      {name}
    </Text>
  );
}

// Indent: sub-nível com a borda à esquerda que dá a noção de hierarquia.
function Indent({ children }: { children: React.ReactNode }) {
  return (
    <Box pl="sm" style={{ borderLeft: "2px solid var(--mantine-color-default-border)" }}>
      {children}
    </Box>
  );
}

function Node({ value }: { value: unknown }) {
  if (isScalar(value)) return <Leaf value={value} />;

  if (Array.isArray(value)) {
    if (value.every(isScalar)) {
      return (
        <Group gap={6}>
          {value.length === 0 && (
            <Text size="sm" c="dimmed" fs="italic">
              vazio
            </Text>
          )}
          {value.map((v, i) => (
            <Badge key={i} size="sm" variant="light" color="blue" tt="none">
              {String(v)}
            </Badge>
          ))}
        </Group>
      );
    }
    return (
      <Stack gap="xs">
        {value.map((v, i) => (
          <Group key={i} gap="xs" align="flex-start" wrap="nowrap">
            <Badge size="sm" variant="outline" color="gray" style={{ flexShrink: 0 }}>
              #{i + 1}
            </Badge>
            <Node value={v} />
          </Group>
        ))}
      </Stack>
    );
  }

  // objeto: escalares/arrays-de-escalares como linhas; aninhados como sub-seções
  const entries = Object.entries(value as Record<string, unknown>);
  if (entries.length === 0)
    return (
      <Text size="sm" c="dimmed" fs="italic">
        vazio
      </Text>
    );
  return (
    <Stack gap={6}>
      {entries.map(([k, v]) =>
        isScalar(v) || (Array.isArray(v) && v.every(isScalar)) ? (
          <Group key={k} gap="sm" align="baseline" wrap="nowrap">
            <Box w={130} style={{ flexShrink: 0 }}>
              <KeyLabel name={k} />
            </Box>
            <Node value={v} />
          </Group>
        ) : (
          <Stack key={k} gap={4}>
            <KeyLabel name={k} />
            <Indent>
              <Node value={v} />
            </Indent>
          </Stack>
        ),
      )}
    </Stack>
  );
}

export function JsonMaterialized({ value }: { value: unknown }) {
  return <Node value={value} />;
}
