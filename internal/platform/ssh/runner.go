package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/teranga-host/terangahost/internal/domain"
	"golang.org/x/crypto/ssh"
)

// NativeSSHRunner implémente domain.Runner via une connexion SSH active
type NativeSSHRunner struct {
	client *ssh.Client
}

// NewNativeSSHRunner instancie le runner
func NewNativeSSHRunner(client *ssh.Client) *NativeSSHRunner {
	return &NativeSSHRunner{client: client}
}

// Execute exécute une commande distante avec injection des variables non-interactives et stream des flux
func (r *NativeSSHRunner) Execute(ctx context.Context, cmd string, stdout, stderr io.Writer) error {
	session, err := r.client.NewSession()
	if err != nil {
		return fmt.Errorf("impossible de créer la session SSH: %w", err)
	}
	defer session.Close()

	if stdout != nil {
		session.Stdout = stdout
	}
	if stderr != nil {
		session.Stderr = stderr
	}

	// Wrapper strict pour forcer le mode non-interactif et éviter les blocages de debconf
	wrappedCmd := fmt.Sprintf("export DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a UCF_FORCE_CONFFOLD=1; %s", cmd)

	// Gestion de l'interruption par Context (ex: Ctrl+C)
	done := make(chan error, 1)
	go func() {
		done <- session.Run(wrappedCmd)
	}()

	select {
	case <-ctx.Done():
		// Signal d'arrêt reçu : on tente de terminer la session proprement
		_ = session.Signal(ssh.SIGINT)
		_ = session.Close()
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%w: %v (commande: %s)", domain.ErrCommandExecution, err, cmd)
		}
		return nil
	}
}

// RunSilent exécute une commande et retourne la sortie texte
func (r *NativeSSHRunner) RunSilent(ctx context.Context, cmd string) (string, error) {
	var stdout, stderr bytes.Buffer
	err := r.Execute(ctx, cmd, &stdout, &stderr)
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		errOut := strings.TrimSpace(stderr.String())
		if errOut != "" {
			return out, fmt.Errorf("%w: %s", err, errOut)
		}
		return out, err
	}
	return out, nil
}

// Upload téléverse un contenu de fichier directement via stdin / cat
func (r *NativeSSHRunner) Upload(ctx context.Context, content []byte, remotePath string, mode uint32) error {
	session, err := r.client.NewSession()
	if err != nil {
		return fmt.Errorf("impossible de créer la session SSH pour upload: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf("cat > '%s' && chmod %04o '%s'", remotePath, mode, remotePath)

	done := make(chan error, 1)
	go func() {
		done <- session.Run(cmd)
	}()

	_, writeErr := stdin.Write(content)
	closeErr := stdin.Close()

	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// FileExists vérifie l'existence d'un fichier ou répertoire distant
func (r *NativeSSHRunner) FileExists(ctx context.Context, remotePath string) (bool, error) {
	cmd := fmt.Sprintf("[ -e '%s' ] && echo 'EXISTS' || echo 'MISSING'", remotePath)
	out, err := r.RunSilent(ctx, cmd)
	if err != nil {
		return false, err
	}
	return strings.Contains(out, "EXISTS"), nil
}

// Close ferme la connexion SSH
func (r *NativeSSHRunner) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
