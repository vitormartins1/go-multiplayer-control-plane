-- Dados de gameplay por jogador (≈ GameKit User Gameplay Data).
-- Chave-valor por namespace; value é JSON opaco (JSONB). Banco próprio do
-- serviço (ADR-0015). player_id é TEXT (sub do JWT; aceita guest-<uuid>).
CREATE TABLE player_data (
    player_id  TEXT        NOT NULL,
    namespace  TEXT        NOT NULL,
    key        TEXT        NOT NULL,
    value      JSONB       NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (player_id, namespace, key)
);
