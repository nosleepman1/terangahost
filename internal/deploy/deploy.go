// Package deploy réalise les déploiements zero-downtime et les retours arrière d'un site Laravel.
package deploy

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/shell"
	"github.com/nosleepman1/terangahost/templates"
)

// Options d'un déploiement.
type Options struct {
	Repository   string
	Branch       string
	Migrate      bool
	KeepReleases int
}

// Events reçoit la progression du déploiement (toutes les fonctions sont optionnelles).
type Events struct {
	OnStep   func(title string)
	OnWarn   func(msg string)
	OnOutput func(line string) // sortie brute des commandes (mode verbeux)
}

// Result décrit un déploiement.
type Result struct {
	Release   string
	Commit    string
	Activated bool
	Warnings  []string
	Output    []string // dernières lignes de sortie (diagnostic en cas d'échec)
}

type scriptData struct {
	Domain, Directory, ReleaseID, PHPBinary, FPMService, Repository, Branch string
	Migrate, Reverb                                                         bool
	KeepReleases                                                            int
}

// NewReleaseID retourne un identifiant de release horodaté (UTC, tri lexical = tri chronologique).
func NewReleaseID(t time.Time) string {
	return t.UTC().Format("20060102150405")
}

// Script génère le script bash de déploiement exécuté sur le serveur.
func Script(s *domain.Site, releaseID string, opts Options) ([]byte, error) {
	keep := opts.KeepReleases
	if keep < 1 {
		keep = 5
	}
	return templates.Render("deploy/deploy.sh.tmpl", scriptData{
		Domain:       s.Domain,
		Directory:    s.Directory,
		ReleaseID:    releaseID,
		PHPBinary:    "php" + s.PHPVersion,
		FPMService:   "php" + s.PHPVersion + "-fpm",
		Repository:   opts.Repository,
		Branch:       opts.Branch,
		Migrate:      opts.Migrate,
		Reverb:       s.WithReverb,
		KeepReleases: keep,
	})
}

// Run déploie une nouvelle release. Le runner doit être connecté en tant que deployer.
// En cas d'échec avant l'activation, la release est supprimée et la production reste inchangée.
func Run(ctx context.Context, r domain.Runner, s *domain.Site, opts Options, ev Events) (*Result, error) {
	release := NewReleaseID(time.Now())
	script, err := Script(s, release, opts)
	if err != nil {
		return nil, err
	}
	res := &Result{Release: release}
	scriptPath := s.Directory + "/shared/.terangahost-deploy-" + release + ".sh"
	if err := r.Upload(ctx, script, scriptPath, 0o700); err != nil {
		return res, fmt.Errorf("envoi du script de déploiement: %w", err)
	}

	w := newMarkerWriter(res, ev)
	q := shell.Quote(scriptPath)
	runErr := r.Execute(ctx, "bash "+q+"; rc=$?; rm -f "+q+"; exit $rc", w, w)
	w.Flush()
	res.Output = w.tail()
	if runErr != nil {
		return res, runErr
	}
	return res, nil
}

// markerWriter découpe la sortie en lignes et interprète les marqueurs "::th-*::" du script.
type markerWriter struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	res   *Result
	ev    Events
	lines []string
}

func newMarkerWriter(res *Result, ev Events) *markerWriter {
	return &markerWriter{res: res, ev: ev}
}

func (w *markerWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// Ligne incomplète : on la remet dans le tampon.
			w.buf.Reset()
			w.buf.WriteString(line)
			break
		}
		w.handle(strings.TrimRight(line, "\r\n"))
	}
	return len(p), nil
}

// Flush traite une éventuelle dernière ligne sans retour chariot.
func (w *markerWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() > 0 {
		w.handle(w.buf.String())
		w.buf.Reset()
	}
}

func (w *markerWriter) handle(line string) {
	if rest, ok := strings.CutPrefix(line, "::th-"); ok {
		kind, value, _ := strings.Cut(rest, "::")
		switch kind {
		case "step":
			if w.ev.OnStep != nil {
				w.ev.OnStep(value)
			}
			return
		case "warn":
			w.res.Warnings = append(w.res.Warnings, value)
			if w.ev.OnWarn != nil {
				w.ev.OnWarn(value)
			}
			return
		case "commit":
			w.res.Commit = value
			return
		case "activated":
			w.res.Activated = true
			return
		case "done":
			return
		}
	}
	w.lines = append(w.lines, line)
	if len(w.lines) > 60 {
		w.lines = w.lines[len(w.lines)-60:]
	}
	if w.ev.OnOutput != nil {
		w.ev.OnOutput(line)
	}
}

func (w *markerWriter) tail() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.lines...)
}

var releaseRe = regexp.MustCompile(`^[0-9]{14}$`)

// Releases liste les releases présentes (de la plus récente à la plus ancienne) et la release active.
func Releases(ctx context.Context, r domain.Runner, s *domain.Site) (releases []string, current string, err error) {
	dir := shell.Quote(s.Directory)
	out, err := r.RunSilent(ctx, "ls -1 "+dir+"/releases 2>/dev/null || true; echo '--current--'; "+
		"[ -L "+dir+"/current ] && basename \"$(readlink -f "+dir+"/current)\" || true")
	if err != nil {
		return nil, "", err
	}
	list, cur, _ := strings.Cut(out, "--current--")
	for _, l := range strings.Split(list, "\n") {
		if l = strings.TrimSpace(l); releaseRe.MatchString(l) {
			releases = append(releases, l)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(releases)))
	return releases, strings.TrimSpace(cur), nil
}

// PreviousRelease retourne la release immédiatement antérieure à la release active.
func PreviousRelease(releases []string, current string) (string, error) {
	for i, rel := range releases {
		if rel == current {
			if i+1 < len(releases) {
				return releases[i+1], nil
			}
			return "", fmt.Errorf("aucune release antérieure à %s", current)
		}
	}
	if current == "" {
		return "", fmt.Errorf("aucune release active")
	}
	return "", fmt.Errorf("release active %s introuvable dans releases/", current)
}

// Activate bascule atomiquement le lien current vers une release existante puis recharge l'application.
func Activate(ctx context.Context, r domain.Runner, s *domain.Site, release string) error {
	if !releaseRe.MatchString(release) {
		return fmt.Errorf("%w: identifiant de release %q", domain.ErrInvalidInput, release)
	}
	dir := shell.Quote(s.Directory)
	target := s.Directory + "/releases/" + release
	php := "php" + s.PHPVersion
	cmd := fmt.Sprintf("[ -d %[1]s ] || { echo 'release %[2]s introuvable' >&2; exit 1; }; "+
		"ln -sfn %[1]s %[3]s/.current.tmp && mv -Tf %[3]s/.current.tmp %[3]s/current",
		shell.Quote(target), release, dir)
	if _, err := r.RunSilent(ctx, cmd); err != nil {
		return err
	}
	_, _ = r.RunSilent(ctx, fmt.Sprintf("sudo -n /usr/bin/systemctl reload php%s-fpm || true; cd %s/current && %s artisan config:cache >/dev/null 2>&1; %s artisan queue:restart >/dev/null 2>&1 || true",
		s.PHPVersion, dir, php, php))
	return nil
}
