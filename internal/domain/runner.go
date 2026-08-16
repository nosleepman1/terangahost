package domain

import (
	"context"
	"io"
)

// Runner est le contrat d'exécution de commandes et d'envoi de fichiers à distance ou en local.
// Cette abstraction permet le mocking complet dans les tests sans avoir besoin d'un VPS réel.
type Runner interface {
	// Execute exécute une commande sur le système distant et stream stdout/stderr
	Execute(ctx context.Context, cmd string, stdout, stderr io.Writer) error

	// RunSilent exécute une commande et retourne la sortie brute (sans affichage)
	RunSilent(ctx context.Context, cmd string) (string, error)

	// Upload envoie un contenu de fichier vers le système distant avec des permissions données
	Upload(ctx context.Context, content []byte, remotePath string, mode uint32) error

	// FileExists vérifie si un fichier ou dossier distant existe
	FileExists(ctx context.Context, remotePath string) (bool, error)

	// Close ferme proprement les connexions et sockets
	Close() error
}
