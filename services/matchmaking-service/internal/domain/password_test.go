package domain_test

import (
	"testing"

	"github.com/vitormartins1/go-multiplayer-control-plane/services/matchmaking-service/internal/domain"
)

func TestHashWorldPassword_VazioDevolveVazio(t *testing.T) {
	if got := domain.HashWorldPassword(""); got != "" {
		t.Fatalf("HashWorldPassword(\"\") = %q, want \"\"", got)
	}
}

func TestHashWorldPassword_Deterministico(t *testing.T) {
	a := domain.HashWorldPassword("abrepalavra")
	b := domain.HashWorldPassword("abrepalavra")
	if a != b {
		t.Fatalf("hashes diferentes pra mesma entrada: %q != %q", a, b)
	}
	if a == "" {
		t.Fatal("hash de senha não-vazia não pode ser vazio")
	}
}

func TestHashWorldPassword_SenhasDiferentesGeramHashesDiferentes(t *testing.T) {
	a := domain.HashWorldPassword("abrepalavra")
	b := domain.HashWorldPassword("outrasenha")
	if a == b {
		t.Fatal("senhas diferentes geraram o mesmo hash")
	}
}
