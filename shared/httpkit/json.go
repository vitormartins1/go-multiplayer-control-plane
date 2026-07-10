// Package httpkit centraliza helpers pequenos e sem estado de resposta HTTP
// compartilhados pelos transports dos serviços (ADR-0002).
package httpkit

import (
	"encoding/json"
	"net/http"
)

// WriteJSON escreve v como corpo JSON com o status informado. Erro de encode
// é ignorado de propósito: a essa altura o header/status já foi escrito, não
// há como sinalizar erro ao cliente por outro canal.
//
// Achado no /code-review: reimplementada identicamente (byte a byte) em 8
// serviços antes de centralizar aqui.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError escreve o corpo de erro padrão do repo: {"error": "<code>"}.
// code é um identificador estável (snake_case), não a mensagem de erro Go —
// consumido pelos clientes (admin-bff, admin-ui) para diferenciar motivos.
func WriteError(w http.ResponseWriter, status int, code string) {
	WriteJSON(w, status, map[string]string{"error": code})
}
