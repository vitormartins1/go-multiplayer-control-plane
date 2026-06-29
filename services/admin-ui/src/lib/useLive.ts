import { useCallback, useEffect, useRef, useState } from "react";

export interface RefreshOptions {
  enabled: boolean; // auto-refresh ligado?
  intervalMs: number; // 5000 / 15000 / 30000
}

export interface LiveState<T> {
  data: T | undefined; // último dado bom (atualização suave: não some em erro pontual)
  error: unknown;
  loading: boolean; // só na primeira carga
  reload: () => void;
}

// useLive busca via fetcher e, se enabled, re-busca a cada intervalo. Mantém o
// último dado bom para a tela não piscar; erro pontual não apaga o que há.
export function useLive<T>(
  fetcher: (signal: AbortSignal) => Promise<T>,
  deps: unknown[],
  opts: RefreshOptions,
): LiveState<T> {
  const [data, setData] = useState<T>();
  const [error, setError] = useState<unknown>(undefined);
  const [loading, setLoading] = useState(true);
  const hasData = useRef(false);

  // fetcher muda toda render; guardamos a versão atual sem reassinar o efeito.
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;

  const run = useCallback((signal: AbortSignal) => {
    if (!hasData.current) setLoading(true);
    return fetcherRef.current(signal)
      .then((d) => {
        setData(d);
        setError(undefined);
        hasData.current = true;
      })
      .catch((e) => {
        if (signal.aborted) return;
        setError(e); // mantém o data anterior (atualização suave)
      })
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // chave de reload manual
  const [tick, setTick] = useState(0);
  const reload = useCallback(() => setTick((t) => t + 1), []);

  useEffect(() => {
    const ac = new AbortController();
    void run(ac.signal);
    let timer: number | undefined;
    if (opts.enabled) {
      timer = window.setInterval(() => {
        const cy = new AbortController();
        void run(cy.signal);
      }, opts.intervalMs);
    }
    return () => {
      ac.abort();
      if (timer) window.clearInterval(timer);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, opts.enabled, opts.intervalMs, tick]);

  return { data, error, loading, reload };
}
