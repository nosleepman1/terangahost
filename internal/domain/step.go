package domain

import "context"

// Step définit le contrat d'une étape de provisionnement ou de maintenance du serveur.
// Chaque étape DOIT être 100% idempotente.
type Step interface {
	// ID renvoie l'identifiant technique unique de l'étape (ex: "php_installation")
	ID() string

	// Title renvoie le libellé clair affiché dans le terminal (ex: "Installation de PHP 8.3 & Extensions FPM")
	Title() string

	// PreCheck vérifie si l'étape a déjà été exécutée avec succès pour éviter les opérations redondantes
	// Retourne true si l'étape est déjà satisfaite (elle sera alors sautée avec le statut [SKIPPED])
	PreCheck(ctx context.Context, r Runner, s *Server) (bool, error)

	// Execute applique les configurations nécessaires sur le serveur
	Execute(ctx context.Context, r Runner, s *Server) error
}
