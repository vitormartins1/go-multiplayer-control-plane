import { useEffect, useRef } from "react";

// usePolling (spec 084): dispara `callback` a cada `ms` — ms<=0 (settings.tsx,
// pollingMs "Pausado") desliga o polling inteiramente. Um lugar só pra esse
// padrão, que antes cada página reimplementava com seu próprio setInterval e
// seu próprio intervalo fixo (4 valores diferentes sem motivo real).
//
// callback guardado em ref: o interval sempre chama a versão mais recente,
// então o efeito só precisa re-assinar quando `ms` muda — não é preciso
// listar as dependências do callback (gameId etc.) na array de efeito, ao
// contrário do padrão anterior (que fixava isso com eslint-disable).
export function usePolling(callback: () => void, ms: number) {
  const callbackRef = useRef(callback);
  useEffect(() => {
    callbackRef.current = callback;
  });

  useEffect(() => {
    if (ms <= 0) return;
    const id = setInterval(() => callbackRef.current(), ms);
    return () => clearInterval(id);
  }, [ms]);
}
