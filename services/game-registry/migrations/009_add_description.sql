-- Descrição curta do jogo (spec 013, revisão de layout do admin-ui). Nullable:
-- jogo nasce sem descrição; NULL/ausente é tratado como "" na leitura, mesma
-- convenção do fleet (migration 007).
ALTER TABLE games ADD COLUMN description TEXT;
