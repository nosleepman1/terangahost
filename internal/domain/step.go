package domain

import "context"

// Step définit le contrat d'une étape de provisionnement. Chaque étape DOIT être idempotente :
// elle peut être rejouée sans effet de bord sur un serveur déjà configuré.
type Step interface {
	// ID renvoie l'identifiant technique unique de l'étape.
	ID() string

	// Title renvoie le libellé affiché dans le terminal.
	Title() string

	// PreCheck retourne true si l'étape est déjà satisfaite (elle est alors sautée).
	PreCheck(ctx context.Context, r Runner, s *Server) (bool, error)

	// Execute applique la configuration sur le serveur.
	Execute(ctx context.Context, r Runner, s *Server) error
}
