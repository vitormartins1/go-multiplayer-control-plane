-- Controle de concorrência otimista pra SetModes (achado no /code-review,
-- item 3 — revisão do trade-off "last-write-wins aceito" da spec 009/plan.md:
-- o read-modify-write de SetMode/DeleteMode perdia silenciosamente uma
-- edição concorrente sem nenhum sinal pro admin. Em vez de lock, um
-- contador incrementado a cada SetModes bem-sucedido, comparado no WHERE —
-- a escrita que perde a corrida falha com erro claro (ErrModeConflict) em
-- vez de sumir sem avisar.
ALTER TABLE games ADD COLUMN multiplayer_profile_version BIGINT NOT NULL DEFAULT 1;
