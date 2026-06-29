import { useEffect, useState } from "react";

export interface AsyncState<T> {
  data: T | undefined;
  error: unknown;
  loading: boolean;
  reload: () => void;
}

// useAsync carrega ao montar (e quando deps mudam) e expõe reload() para o
// botão "Atualizar". Sem auto-refresh.
export function useAsync<T>(fn: (signal: AbortSignal) => Promise<T>, deps: unknown[]): AsyncState<T> {
  const [data, setData] = useState<T>();
  const [error, setError] = useState<unknown>();
  const [loading, setLoading] = useState(true);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    const ac = new AbortController();
    setLoading(true);
    fn(ac.signal)
      .then((d) => {
        setData(d);
        setError(undefined);
      })
      .catch((e) => {
        if (!ac.signal.aborted) setError(e);
      })
      .finally(() => setLoading(false));
    return () => ac.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, tick]);

  return { data, error, loading, reload: () => setTick((t) => t + 1) };
}
