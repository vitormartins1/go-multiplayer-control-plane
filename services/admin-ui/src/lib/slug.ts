// slugify deriva um slug válido para o game-registry a partir do nome do jogo
// (035): minúsculas, acentos removidos, qualquer separador vira hífen.
// Regra do backend: ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$
export function slugify(name: string): string {
  return name
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "") // remove diacríticos (Ágil -> Agil)
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}
