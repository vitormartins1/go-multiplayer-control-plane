import JsonView from "react18-json-view";
import "react18-json-view/src/style.css";
import "react18-json-view/src/dark.css";
import { useComputedColorScheme } from "@mantine/core";

// JsonTree: visualização interativa read-only de JSON (spec 042) — expande/
// colapsa chaves e arrays, sem edição. Wrapper fino do react18-json-view para
// seguir o color scheme do console e fixar os defaults num lugar só:
//   enableClipboard  — copiar valor/subárvore (interação sem edição);
//   displaySize      — nó colapsado mostra a contagem ("5 items");
//   collapseStrings  — strings longas truncam com "..." expansível.
export function JsonTree({ value, collapsed = 2 }: { value: unknown; collapsed?: number | boolean }) {
  const scheme = useComputedColorScheme("dark");
  return (
    <JsonView
      src={value}
      dark={scheme === "dark"}
      collapsed={collapsed}
      enableClipboard
      displaySize="collapsed"
      collapseStringsAfterLength={60}
    />
  );
}
