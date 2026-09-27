package domain

import (
	"context"
	"io"
)

// Runner est le contrat d'exécution de commandes et d'envoi de fichiers sur le serveur.
// Cette abstraction permet de tester toute la logique sans VPS réel.
type Runner interface {
	// Execute exécute une commande bash et stream stdout/stderr.
	Execute(ctx context.Context, cmd string, stdout, stderr io.Writer) error

	// RunSilent exécute une commande et retourne sa sortie standard (sans espaces de bord).
	RunSilent(ctx context.Context, cmd string) (string, error)

	// RunWithInput exécute une commande en lui fournissant input sur l'entrée standard.
	// À utiliser pour tout contenu secret (SQL avec mots de passe...) afin qu'il n'apparaisse
	// ni dans la liste des processus ni dans les journaux.
	RunWithInput(ctx context.Context, cmd string, input []byte) (string, error)

	// Upload écrit atomiquement un fichier distant avec les permissions données.
	Upload(ctx context.Context, content []byte, remotePath string, mode uint32) error

	// FileExists vérifie si un fichier ou dossier distant existe.
	FileExists(ctx context.Context, remotePath string) (bool, error)

	// Close ferme la connexion.
	Close() error
}
