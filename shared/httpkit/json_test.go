package httpkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON_EscreveStatusContentTypeECorpo(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusCreated, map[string]string{"a": "b"})

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, quer %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, quer application/json", ct)
	}
	var got map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("corpo não é JSON válido: %v", err)
	}
	if got["a"] != "b" {
		t.Fatalf("corpo = %v, quer {a: b}", got)
	}
}

func TestWriteError_CorpoPadraoErrorCode(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusNotFound, "not_found")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quer %d", w.Code, http.StatusNotFound)
	}
	var got map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("corpo não é JSON válido: %v", err)
	}
	if got["error"] != "not_found" {
		t.Fatalf("corpo = %v, quer {error: not_found}", got)
	}
}
