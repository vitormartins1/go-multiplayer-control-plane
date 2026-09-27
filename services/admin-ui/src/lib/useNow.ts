import { useEffect, useState } from "react";

// useNow devolve o relógio atual e re-renderiza a cada `everyMs` — para
// indicadores que envelhecem sozinhos (ex.: o sinal de vida do mundo continua
// passando de "vivo" para "sem sinal" mesmo com o polling pausado).
export function useNow(everyMs: number): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), everyMs);
    return () => window.clearInterval(id);
  }, [everyMs]);
  return now;
}
