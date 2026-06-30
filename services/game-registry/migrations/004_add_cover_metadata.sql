-- 004: metadados da capa do jogo (feature 003-generated-cover).
-- Sem upload no MVP: a capa é GERADA no cliente a partir de uma seed
-- determinística (default = slug). O backend guarda só os metadados; `image`
-- fica reservado para upload futuro (NULL = sem upload).
ALTER TABLE games ADD COLUMN image                 TEXT;
ALTER TABLE games ADD COLUMN image_source          TEXT NOT NULL DEFAULT 'generated';
ALTER TABLE games ADD COLUMN generated_image_seed  TEXT;
ALTER TABLE games ADD COLUMN generated_image_style TEXT NOT NULL DEFAULT 'shapes';

-- Backfill: registros antigos passam a usar o slug como seed (mesma regra do
-- cadastro novo). image_source/style já vêm pelo DEFAULT.
UPDATE games SET generated_image_seed = slug WHERE generated_image_seed IS NULL;
