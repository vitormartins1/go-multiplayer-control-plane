-- Soft delete de jogo (ADR-0025): deleted_at NULL = vivo. O registry é o
-- ponto de corte único — jogo deletado responde como inexistente em toda a
-- API (filtro no repository); o dado satélite dos demais serviços fica órfão
-- e inacessível. Sem índice: catálogo pequeno, buscas por PK.
ALTER TABLE games ADD COLUMN deleted_at TIMESTAMPTZ;
