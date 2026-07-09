package domain

// Event types de gameplay derivados dos eventos de participação do
// host_world_dropin (spec 013/009). Devem casar com o event_type cadastrado no
// catálogo (data-driven) — o operador cria conquistas com esses gatilhos.
// O MAPEAMENTO papel×evento×outcome -> event type virou dado (migration 008,
// tabela event_rules, EvaluateRules em event_rule.go) — estas consts só
// nomeiam o vocabulário de saída, referenciado pelo seed da migration e por
// quem cria achievements/regras novas.
const (
	EventInvasionWon    = "invasion_won"     // venceu como invasor (matou o host)
	EventCoopBossHelped = "coop_boss_helped" // ajudou a matar o boss como co-op
	EventWorldJoined    = "world_joined"     // entrou num mundo (qualquer papel)
)
