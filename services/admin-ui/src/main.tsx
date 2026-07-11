import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { MantineProvider, createTheme } from "@mantine/core";
import { Notifications } from "@mantine/notifications";

import "@mantine/core/styles.css";
import "@mantine/notifications/styles.css";
import "@mantine/charts/styles.css";
import "@xyflow/react/dist/style.css";

// Sistema tipográfico (POC de redesign — painel de instrumentos/centro de
// controle, self-hosted pra não depender de rede externa, projeto local-first):
// Space Grotesk = display (títulos, nav) · IBM Plex Sans = corpo · IBM Plex
// Mono = todo dado numérico/id/tag, estendendo o monospace que o kit dos
// canvas (flow-card) já usa isoladamente pro app inteiro.
import "@fontsource/space-grotesk/500.css";
import "@fontsource/space-grotesk/600.css";
import "@fontsource/space-grotesk/700.css";
import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/ibm-plex-sans/600.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";

import "./index.css";

import { App } from "./App";

// Paleta "painel de instrumentos" (POC de redesign, ver conversa): fundo
// quase-preto com tinta azul-slate (não o cinza neutro padrão da Mantine) +
// três cores semânticas de status (não decorativas — já existem estados reais
// no app: saúde de serviço, health de fleet) + um accent de interação
// separado dos status (signal).
const theme = createTheme({
  primaryColor: "signal",
  fontFamily: "'IBM Plex Sans', ui-sans-serif, system-ui, sans-serif",
  fontFamilyMonospace: "'IBM Plex Mono', ui-monospace, monospace",
  headings: {
    fontFamily: "'Space Grotesk', sans-serif",
    fontWeight: "600",
  },
  colors: {
    // Substitui a escala neutra padrão da Mantine — mesma convenção de índice
    // (0 claro -> 9 escuro), só com tinta azul-slate. dark[6] alinhado ao
    // hairline já usado pelo kit dos canvas (flow.css, #2a3340).
    dark: [
      "#E6EDF3",
      "#C8D3DE",
      "#9AA7B6",
      "#7D8A99",
      "#5C6B7C",
      "#3A4657",
      "#2A3340",
      "#1A212B",
      "#12161D",
      "#0A0D12",
    ],
    signal: [
      "#E7F5FF",
      "#CFE9FF",
      "#9FD3FF",
      "#6FBDFF",
      "#5EB8FF",
      "#3FA3F0",
      "#2E8AD1",
      "#2270AC",
      "#175586",
      "#0F3A5E",
    ],
    nominal: [
      "#E3FCEF",
      "#C3F7DC",
      "#96EEC1",
      "#69E4A6",
      "#4ADE9A",
      "#33C384",
      "#26A570",
      "#1C875C",
      "#136B48",
      "#0B4F35",
    ],
    critical: [
      "#FFECEC",
      "#FFD1D1",
      "#FFA3A3",
      "#FF7A7A",
      "#FF5C5C",
      "#F03E3E",
      "#D62828",
      "#B01E1E",
      "#8A1717",
      "#651010",
    ],
  },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <MantineProvider theme={theme} defaultColorScheme="dark">
      <Notifications />
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </MantineProvider>
  </StrictMode>,
);
