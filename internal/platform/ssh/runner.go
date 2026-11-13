package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/shell"
	"golang.org/x/crypto/ssh"
)

// RunnerOptions configure l'exécution des commandes.
type RunnerOptions struct {
	// Sudo exécute chaque commande via "sudo -n" (utilisateur d'administration non root).
	Sudo bool
	// Log reçoit chaque commande et sa sortie (journal détaillé sur disque). Peut être nil.
	Log io.Writer
}

// Runner implémente domain.Runner via une connexion SSH active.
type Runner struct {
	client *Client
	opts   RunnerOptions
	logMu  sync.Mutex
}

var _ domain.Runner = (*Runner)(nil)

// NewRunner instancie le runner.
func NewRunner(client *Client, opts RunnerOptions) *Runner {
	return &Runner{client: client, opts: opts}
}

// Client retourne la connexion sous-jacente.
func (r *Runner) Client() *Client { return r.client }

// wrap force bash, le mode non interactif d'APT et pipefail, puis ajoute sudo si nécessaire.
func (r *Runner) wrap(cmd string) string {
	script := "export DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a NEEDRESTART_SUSPEND=1 LC_ALL=C.UTF-8; set -o pipefail; " + cmd
	full := "bash -c " + shell.Quote(script)
	if r.opts.Sudo {
		full = "sudo -n " + full
	}
	return full
}

// logWriter sérialise les écritures concurrentes de stdout/stderr dans le journal.
type logWriter struct{ r *Runner }

func (l logWriter) Write(p []byte) (int, error) {
	l.r.logMu.Lock()
	defer l.r.logMu.Unlock()
	return l.r.opts.Log.Write(p)
}

func (r *Runner) logf(format string, args ...any) {
	if r.opts.Log != nil {
		fmt.Fprintf(logWriter{r}, format, args...)
	}
}

func (r *Runner) run(ctx context.Context, cmd string, stdin io.Reader, stdout, stderr io.Writer) error {
	session, err := r.client.NewSession()
	if err != nil {
		return fmt.Errorf("impossible de créer la session SSH: %w", err)
	}
	defer session.Close()

	r.logf("\n$ %s\n", cmd)
	outs := []io.Writer{}
	errs := []io.Writer{}
	if stdout != nil {
		outs = append(outs, stdout)
	}
	if stderr != nil {
		errs = append(errs, stderr)
	}
	if r.opts.Log != nil {
		outs = append(outs, logWriter{r})
		errs = append(errs, logWriter{r})
	}
	if len(outs) > 0 {
		session.Stdout = io.MultiWriter(outs...)
	}
	if len(errs) > 0 {
		session.Stderr = io.MultiWriter(errs...)
	}
	if stdin != nil {
		session.Stdin = stdin
	}

	done := make(chan error, 1)
	go func() { done <- session.Run(r.wrap(cmd)) }()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		_ = session.Close()
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%w: %v", domain.ErrCommandExecution, err)
		}
		return nil
	}
}

// Execute exécute une commande et stream stdout/stderr vers les writers fournis.
func (r *Runner) Execute(ctx context.Context, cmd string, stdout, stderr io.Writer) error {
	return r.run(ctx, cmd, nil, stdout, stderr)
}

func (r *Runner) capture(ctx context.Context, cmd string, stdin io.Reader) (string, error) {
	var stdout, stderr bytes.Buffer
	err := r.run(ctx, cmd, stdin, &stdout, &stderr)
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		detail := lastLines(strings.TrimSpace(stderr.String()), 15)
		if detail == "" {
			detail = lastLines(out, 15)
		}
		if detail != "" {
			return out, fmt.Errorf("%w\n%s", err, detail)
		}
		return out, err
	}
	return out, nil
}

// RunSilent exécute une commande et retourne sa sortie standard.
func (r *Runner) RunSilent(ctx context.Context, cmd string) (string, error) {
	return r.capture(ctx, cmd, nil)
}

// RunWithInput exécute une commande en lui passant input sur stdin (non journalisé).
func (r *Runner) RunWithInput(ctx context.Context, cmd string, input []byte) (string, error) {
	return r.capture(ctx, cmd, bytes.NewReader(input))
}

// Upload écrit le fichier dans un fichier temporaire du même dossier puis le renomme :
// le fichier final n'est jamais observé à moitié écrit.
func (r *Runner) Upload(ctx context.Context, content []byte, remotePath string, mode uint32) error {
	dir := path.Dir(remotePath)
	cmd := fmt.Sprintf(`mkdir -p %[1]s && tmp=$(mktemp %[1]s/.terangahost.XXXXXX) && cat > "$tmp" && chmod %04[3]o "$tmp" && mv -f "$tmp" %[2]s || { rm -f "$tmp"; exit 1; }`,
		shell.Quote(dir), shell.Quote(remotePath), mode)
	if _, err := r.RunWithInput(ctx, cmd, content); err != nil {
		return fmt.Errorf("écriture de %s: %w", remotePath, err)
	}
	return nil
}

// FileExists vérifie l'existence d'un fichier ou répertoire distant.
func (r *Runner) FileExists(ctx context.Context, remotePath string) (bool, error) {
	out, err := r.RunSilent(ctx, fmt.Sprintf("[ -e %s ] && echo EXISTS || echo MISSING", shell.Quote(remotePath)))
	if err != nil {
		return false, err
	}
	return strings.Contains(out, "EXISTS"), nil
}

// Close ferme la connexion SSH.
func (r *Runner) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
