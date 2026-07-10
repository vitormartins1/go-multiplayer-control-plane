-- Fleet Agones do jogo (ADR-0026, spec 012): nome da Fleet/label
-- `agones.dev/fleet` que o session-service usa para alocar game servers deste
-- jogo. Nullable: jogo nasce sem Fleet, setada depois via PUT /games/{id}/fleet.
-- NULL/ausente é tratado como "" na leitura (repository).
ALTER TABLE games ADD COLUMN fleet TEXT;
