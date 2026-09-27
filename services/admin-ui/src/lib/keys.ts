// escapeOwnedByPopup diz se há um diálogo (Modal) ou um dropdown (Select,
// Combobox, Menu) aberto: o Esc é deles, e os atalhos de Esc da página (sair da
// tela cheia, descartar o rascunho) não devem disparar junto.
export function escapeOwnedByPopup(): boolean {
  return document.querySelector('[role="dialog"], [aria-haspopup][aria-expanded="true"]') !== null;
}
