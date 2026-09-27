# Spec: 095-closed-sessions-live

**Serviço:** admin-ui
**Status:** Done (2026-09-27)
**Data:** 2026-09-27
**Origem:** gravação do vídeo 1. Fatia pequena: um recarregamento a mais na aba Sessions do jogo.

## Contexto

A aba **Sessions** do jogo busca as sessões abertas a cada ciclo de polling, mas o histórico de fechadas (Postgres,
spec 025) só na montagem da aba, porque o código parte da ideia de que "o histórico não muda sozinho". Muda: um
mundo que fecha enquanto o operador olha (host morto, host caiu, encerrado) some de **Abertas** e não aparece em
**Fechadas** até ele sair da aba e voltar. A tela **Rooms** não tem o problema: ela lê abertas e fechadas do mesmo
endpoint, no mesmo polling.

## Comportamento

- Quando uma sessão que estava aberta deixa de aparecer entre as abertas num ciclo de polling, a aba SHALL
  recarregar a primeira página do histórico de fechadas.
- Uma sessão SHALL aparecer uma vez só, mesmo que o recarregamento traga de novo algo que já estava nas páginas
  carregadas por "carregar mais".
- Sem sessão fechando, o histórico SHALL continuar sem polling próprio (nenhuma consulta a mais ao Postgres).

## Critérios de Aceite

1. Com a aba Sessions aberta num mundo ao vivo, reportar `HOST_KILLED`: em até um ciclo de polling, a linha sai de
   Abertas e aparece em Fechadas como "Host morto", sem trocar de aba.
2. Com "carregar mais" já usado, a sessão recém-fechada aparece no topo e nenhuma linha se repete.
3. `npm run lint`, `npm test` e `npm run build` passam.
