package deploy

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
)

func testSite() *domain.Site {
	return &domain.Site{ID: "api_example_com", Domain: "api.example.com", Directory: "/var/www/api.example.com", PHPVersion: "8.3"}
}

func TestScriptSafety(t *testing.T) {
	s := testSite()
	script, err := Script(s, "20260925120000", Options{Repository: "git@github.com:org/app.git", Branch: "main", Migrate: true, KeepReleases: 3})
	if err != nil {
		t.Fatal(err)
	}
	body := string(script)
	must := []string{
		"set -Eeuo pipefail",
		"flock -n 9",
		`"$PHP" artisan migrate --force --no-interaction`,
		`mv -Tf "$SITE_DIR/.current.tmp" "$SITE_DIR/current"`,
		"tail -n +4",
		"sudo -n /usr/bin/systemctl reload php8.3-fpm",
		"--branch main -- git@github.com:org/app.git",
	}
	for _, m := range must {
		if !strings.Contains(body, m) {
			t.Errorf("le script devrait contenir %q", m)
		}
	}
	if strings.Contains(body, "migrate --force || true") {
		t.Error("les erreurs de migration ne doivent jamais être ignorées")
	}

	noMigrate, _ := Script(s, "20260925120000", Options{Repository: "https://x.com/a.git", Branch: "main"})
	if strings.Contains(string(noMigrate), "artisan migrate") {
		t.Error("--no-migrate doit retirer l'étape de migration")
	}
}

func TestScriptQuotesHostileValues(t *testing.T) {
	s := testSite()
	script, err := Script(s, "20260925120000", Options{Repository: "https://x.com/a'; touch /tmp/pwned; '.git", Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), `'https://x.com/a'\''; touch /tmp/pwned; '\''.git'`) {
		t.Errorf("valeur non échappée:\n%s", script)
	}
}

func TestScriptBashSyntax(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash indisponible")
	}
	s := testSite()
	s.WithReverb = true
	script, err := Script(s, "20260925120000", Options{Repository: "https://x.com/a.git", Branch: "main", Migrate: true})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "deploy.sh")
	if err := os.WriteFile(path, script, 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("bash", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("syntaxe bash invalide: %v\n%s", err, out)
	}
}

func TestMarkerWriter(t *testing.T) {
	res := &Result{}
	var steps, warns, output []string
	w := newMarkerWriter(res, Events{
		OnStep:   func(s string) { steps = append(steps, s) },
		OnWarn:   func(s string) { warns = append(warns, s) },
		OnOutput: func(s string) { output = append(output, s) },
	})
	// Écritures fragmentées au milieu des lignes.
	for _, chunk := range []string{"::th-step::Clon", "age\nsortie git\n::th-co", "mmit::abc1234\n::th-warn::oups\n", "::th-activated::1\nfin sans retour"} {
		_, _ = w.Write([]byte(chunk))
	}
	w.Flush()
	if len(steps) != 1 || steps[0] != "Clonage" {
		t.Errorf("steps = %v", steps)
	}
	if res.Commit != "abc1234" || !res.Activated || len(warns) != 1 {
		t.Errorf("résultat incorrect: %+v warns=%v", res, warns)
	}
	if strings.Join(output, "|") != "sortie git|fin sans retour" {
		t.Errorf("output = %v", output)
	}
}

func TestPreviousRelease(t *testing.T) {
	rel := []string{"20260903000000", "20260902000000", "20260901000000"}
	if p, err := PreviousRelease(rel, "20260903000000"); err != nil || p != "20260902000000" {
		t.Errorf("PreviousRelease = %q, %v", p, err)
	}
	if _, err := PreviousRelease(rel, "20260901000000"); err == nil {
		t.Error("aucune release antérieure attendue")
	}
	if _, err := PreviousRelease(rel, ""); err == nil {
		t.Error("erreur attendue sans release active")
	}
}

func TestNewReleaseID(t *testing.T) {
	id := NewReleaseID(time.Date(2026, 9, 25, 13, 4, 5, 0, time.UTC))
	if id != "20260925130405" || !releaseRe.MatchString(id) {
		t.Errorf("id = %s", id)
	}
}
