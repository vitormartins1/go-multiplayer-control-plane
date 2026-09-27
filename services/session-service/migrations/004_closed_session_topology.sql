-- 004_closed_session_topology: o histórico passa a guardar quem hospedou o
-- mundo (spec 031 — topologia p2p). Sem isto, sessões fechadas lidas do
-- Postgres chegavam ao console sem topologia ("—" na coluna Hospedagem).
-- Default vazio: sessões fechadas antes desta migration ficam "desconhecidas".
ALTER TABLE closed_sessions ADD COLUMN topology TEXT NOT NULL DEFAULT '';
ALTER TABLE closed_sessions ADD COLUMN host_connection TEXT NOT NULL DEFAULT '';
