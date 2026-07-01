import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Proxy /api -> admin-bff (default :8088), evitando CORS no dev (ver plan).
// Sobrescreva o alvo com VITE_BFF_URL se o BFF estiver em outra porta/host.
const bff = process.env.VITE_BFF_URL ?? "http://localhost:8088";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": { target: bff, changeOrigin: true },
    },
  },
  build: {
    rollupOptions: {
      output: {
        // Isola os vendors pesados em chunks próprios (cacheáveis à parte e
        // carregados só com as rotas que os usam, via lazy no App).
        manualChunks: {
          dicebear: ["@dicebear/core", "@dicebear/collection"],
          xyflow: ["@xyflow/react"],
          mantine: ["@mantine/core", "@mantine/hooks", "@mantine/form", "@mantine/notifications"],
        },
      },
    },
  },
});
