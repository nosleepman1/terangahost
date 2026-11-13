package deploy

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nosleepman1/terangahost/internal/domain"
)

// Ces tests exécutent réellement le script de déploiement avec bash et les coreutils GNU
// (flock, mv -T...). Seuls git, php et sudo sont simulés. Ils tournent sous Linux (CI).

const gitStub = `#!/bin/sh
if [ "$1" = "clone" ]; then
  for last; do :; done
  mkdir -p "$last/storage/logs" "$last/public" "$last/.git"
  echo '<?php' > "$last/artisan"
  exit 0
fi
[ "$1" = "-C" ] && echo abc1234
exit 0
`

const phpStub = `#!/bin/sh
echo "$*" >> "$STUB_LOG"
case "$*" in *migrate*) if [ -n "$FAIL_MIGRATE" ]; then echo "SQLSTATE[HY000] connexion refusée" >&2; exit 1; fi ;; esac
exit 0
`

const sudoStub = `#!/bin/sh
echo "sudo $*" >> "$STUB_LOG"
exit 0
`

type sandbox struct {
	t       *testing.T
	site    *domain.Site
	env     []string
	logPath string
}

func newSandbox(t *testing.T) *sandbox {
	if runtime.GOOS != "linux" {
		t.Skip("test d'intégration du script réservé à Linux (GNU coreutils)")
	}
	for _, bin := range []string{"bash", "flock"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s indisponible", bin)
		}
	}
	root := t.TempDir()
	stubs := filepath.Join(root, "bin")
	siteDir := filepath.Join(root, "site")
	for _, d := range []string{stubs, siteDir + "/releases", siteDir + "/shared/storage/logs"} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"git": gitStub, "php8.3": phpStub, "sudo": sudoStub} {
		if err := os.WriteFile(filepath.Join(stubs, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(siteDir+"/shared/.env", []byte("APP_KEY=base64:x\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "calls.log")
	return &sandbox{
		t:       t,
		site:    &domain.Site{Domain: "api.example.com", Directory: siteDir, PHPVersion: "8.3"},
		env:     append(os.Environ(), "PATH="+stubs+":"+os.Getenv("PATH"), "STUB_LOG="+logPath),
		logPath: logPath,
	}
}

func (s *sandbox) deploy(release string, extraEnv ...string) (string, error) {
	script, err := Script(s.site, release, Options{Repository: "https://example.com/app.git", Branch: "main", Migrate: true, KeepReleases: 2})
	if err != nil {
		s.t.Fatal(err)
	}
	path := filepath.Join(s.t.TempDir(), "deploy.sh")
	if err := os.WriteFile(path, script, 0o700); err != nil {
		s.t.Fatal(err)
	}
	cmd := exec.Command("bash", path)
	cmd.Env = append(append([]string{}, s.env...), extraEnv...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (s *sandbox) current() string {
	target, err := os.Readlink(filepath.Join(s.site.Directory, "current"))
	if err != nil {
		return ""
	}
	return filepath.Base(target)
}

func TestDeployScriptEndToEnd(t *testing.T) {
	sb := newSandbox(t)

	out, err := sb.deploy("20260101000001")
	if err != nil {
		t.Fatalf("premier déploiement: %v\n%s", err, out)
	}
	if sb.current() != "20260101000001" {
		t.Fatalf("current = %q", sb.current())
	}
	rel := filepath.Join(sb.site.Directory, "releases", "20260101000001")
	if l, _ := os.Readlink(rel + "/.env"); l != sb.site.Directory+"/shared/.env" {
		t.Errorf(".env non lié au fichier partagé: %q", l)
	}
	if l, _ := os.Readlink(rel + "/storage"); l != sb.site.Directory+"/shared/storage" {
		t.Errorf("storage non lié au dossier partagé: %q", l)
	}
	if _, err := os.Stat(rel + "/.git"); !os.IsNotExist(err) {
		t.Error("le dossier .git ne doit pas être conservé")
	}
	for _, marker := range []string{"::th-commit::abc1234", "::th-activated::20260101000001", "::th-done::"} {
		if !strings.Contains(out, marker) {
			t.Errorf("marqueur %q absent", marker)
		}
	}
	calls, _ := os.ReadFile(sb.logPath)
	for _, c := range []string{"install --no-dev", "config:cache", "route:cache", "migrate --force", "queue:restart", "sudo -n /usr/bin/systemctl reload php8.3-fpm"} {
		if !strings.Contains(string(calls), c) {
			t.Errorf("appel attendu: %q", c)
		}
	}

	// Une migration en échec annule la release et laisse la production intacte.
	out, err = sb.deploy("20260101000002", "FAIL_MIGRATE=1")
	if err == nil {
		t.Fatalf("le déploiement doit échouer quand la migration échoue\n%s", out)
	}
	if sb.current() != "20260101000001" {
		t.Errorf("la production a changé malgré l'échec: current=%q", sb.current())
	}
	if _, err := os.Stat(filepath.Join(sb.site.Directory, "releases", "20260101000002")); !os.IsNotExist(err) {
		t.Error("la release en échec doit être supprimée")
	}

	// Rétention : seules les 2 dernières releases sont conservées.
	for _, id := range []string{"20260101000003", "20260101000004", "20260101000005"} {
		if out, err := sb.deploy(id); err != nil {
			t.Fatalf("déploiement %s: %v\n%s", id, err, out)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(sb.site.Directory, "releases"))
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "20260101000004,20260101000005" || sb.current() != "20260101000005" {
		t.Errorf("rétention incorrecte: %v (current=%s)", names, sb.current())
	}
}

func TestDeployScriptLock(t *testing.T) {
	sb := newSandbox(t)
	lock := exec.Command("flock", sb.site.Directory+"/shared/.deploy.lock", "sleep", "5")
	if err := lock.Start(); err != nil {
		t.Fatal(err)
	}
	defer lock.Process.Kill()
	// Laisse flock acquérir le verrou.
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(sb.site.Directory + "/shared/.deploy.lock"); err == nil {
			break
		}
		exec.Command("sleep", "0.05").Run()
	}
	exec.Command("sleep", "0.2").Run()
	out, err := sb.deploy("20260101000009")
	if err == nil || !strings.Contains(out, "deja en cours") {
		t.Fatalf("un second déploiement simultané doit être refusé: %v\n%s", err, out)
	}
	if sb.current() != "" {
		t.Error("aucune release ne doit être activée")
	}
}

func TestMissingEnvAbortsDeploy(t *testing.T) {
	sb := newSandbox(t)
	os.Remove(sb.site.Directory + "/shared/.env")
	out, err := sb.deploy("20260101000010")
	if err == nil || !strings.Contains(out, "site env push") {
		t.Fatalf("un .env absent doit arrêter le déploiement: %v\n%s", err, out)
	}
}
