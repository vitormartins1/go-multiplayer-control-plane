// Package jsonparams extrai e reinjeta chaves "extras" de um objeto JSON —
// o mecanismo do follow-up #14 do arquétipo host_world_dropin: o registry
// decodifica `params` num struct tipado por arquétipo, o que descartava em
// silêncio qualquer chave fora do vocabulário conhecido. Extract preserva
// essas chaves cruas na leitura (PUT); Merge as reinjeta na escrita (GET/
// manifest), sem que o registry precise conhecer o significado delas — o
// data plane é quem lê (ou ignora, se ainda não conhecer o param).
package jsonparams

import "encoding/json"

// Extract devolve as chaves de raw que NÃO estão em known, como texto JSON
// cru (uma string por chave, exatamente como recebida). raw vazio/inválido
// (não é um objeto JSON) devolve nil sem erro — mesma postura best-effort do
// resto da decodificação de params neste serviço.
func Extract(raw []byte, known map[string]struct{}) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil
	}
	var extra map[string]string
	for k, v := range all {
		if _, ok := known[k]; ok {
			continue
		}
		if extra == nil {
			extra = make(map[string]string, len(all))
		}
		extra[k] = string(v)
	}
	return extra
}

// Merge devolve raw com as chaves de extra injetadas (raw já deve ser um
// objeto JSON válido — o shape tipado do arquétipo). Chave de extra que já
// existe em raw é ignorada: o vocabulário tipado do arquétipo sempre manda,
// nunca é sobrescrito por uma chave solta. raw inválido ou extra vazio
// devolve raw como veio.
func Merge(raw []byte, extra map[string]string) []byte {
	if len(extra) == 0 {
		return raw
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		return raw
	}
	for k, v := range extra {
		if _, exists := all[k]; exists {
			continue
		}
		all[k] = json.RawMessage(v)
	}
	merged, err := json.Marshal(all)
	if err != nil {
		return raw
	}
	return merged
}
