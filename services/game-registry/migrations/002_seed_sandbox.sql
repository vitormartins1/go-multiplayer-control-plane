-- Jogo "sandbox": o jogo default do multi-tenant (ADR-0017). Id fixo (UUID nil
-- como sentinela legível) — DEVE casar com o backfill da migração 003 do
-- achievements. Serve para os dados pré-multi-tenant apontarem para um jogo real.
INSERT INTO games (id, slug, name, created_at)
VALUES ('00000000-0000-0000-0000-000000000000', 'sandbox', 'Sandbox', now())
ON CONFLICT DO NOTHING;
