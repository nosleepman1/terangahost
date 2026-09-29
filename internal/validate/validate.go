// Package validate contrôle les paramètres saisis par l'utilisateur avant qu'ils ne
// soient utilisés dans des chemins, des fichiers de configuration ou des commandes distantes.
package validate

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/nosleepman1/terangahost/internal/domain"
)

var (
	labelRe      = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	serverNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	branchRe     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`)
	scpRepoRe    = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:[A-Za-z0-9._~/-]+$`)
	emailRe      = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// SupportedPHPVersions liste les versions PHP installables.
var SupportedPHPVersions = []string{"8.2", "8.3", "8.4"}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalidInput, fmt.Sprintf(format, args...))
}

// Domain normalise (minuscules) et valide un nom de domaine complet.
func Domain(d string) (string, error) {
	d = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(d)), ".")
	if d == "" {
		return "", invalid("le domaine est vide")
	}
	if len(d) > 253 {
		return "", invalid("le domaine %q est trop long", d)
	}
	labels := strings.Split(d, ".")
	if len(labels) < 2 {
		return "", invalid("%q n'est pas un nom de domaine complet (ex: api.monprojet.sn)", d)
	}
	for _, l := range labels {
		if !labelRe.MatchString(l) {
			return "", invalid("%q n'est pas un nom de domaine valide", d)
		}
	}
	if net.ParseIP(d) != nil {
		return "", invalid("utilisez un nom de domaine, pas une adresse IP")
	}
	return d, nil
}

// ServerName valide le nom local d'un serveur.
func ServerName(n string) error {
	if !serverNameRe.MatchString(n) {
		return invalid("nom de serveur %q invalide (minuscules, chiffres, '-' et '_', 32 caractères max)", n)
	}
	return nil
}

// Host valide une adresse IP ou un nom d'hôte.
func Host(h string) error {
	if net.ParseIP(h) != nil {
		return nil
	}
	if _, err := Domain(h); err != nil {
		return invalid("%q n'est ni une adresse IP ni un nom d'hôte valide", h)
	}
	return nil
}

// Port valide un port TCP.
func Port(p int) error {
	if p < 1 || p > 65535 {
		return invalid("port %d hors limites", p)
	}
	return nil
}

// PHPVersion valide une version PHP supportée.
func PHPVersion(v string) error {
	for _, s := range SupportedPHPVersions {
		if v == s {
			return nil
		}
	}
	return invalid("version PHP %q non supportée (valeurs: %s)", v, strings.Join(SupportedPHPVersions, ", "))
}

// Database normalise et valide le moteur de base de données.
func Database(db string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(db)) {
	case "mariadb", "mysql":
		return "mariadb", nil
	case "postgres", "postgresql", "pgsql":
		return "postgres", nil
	case "none", "":
		return "none", nil
	}
	return "", invalid("base de données %q non supportée (mariadb, postgres, none)", db)
}

// Branch valide un nom de branche ou de tag Git.
func Branch(b string) error {
	if !branchRe.MatchString(b) || strings.Contains(b, "..") || strings.HasSuffix(b, ".lock") ||
		strings.HasSuffix(b, "/") || strings.Contains(b, "//") {
		return invalid("nom de branche %q invalide", b)
	}
	return nil
}

// Repository valide une URL de dépôt Git (https://, ssh:// ou git@hôte:chemin).
func Repository(r string) error {
	if r == "" {
		return invalid("l'URL du dépôt est vide")
	}
	if strings.ContainsAny(r, " \t\r\n'\"`$;|&<>\\") || strings.HasPrefix(r, "-") {
		return invalid("URL de dépôt %q invalide", r)
	}
	switch {
	case strings.HasPrefix(r, "https://"), strings.HasPrefix(r, "ssh://"):
		return nil
	case scpRepoRe.MatchString(r):
		return nil
	case strings.HasPrefix(r, "http://"):
		return invalid("dépôt en http:// non chiffré refusé, utilisez https:// ou SSH")
	}
	return invalid("URL de dépôt %q non reconnue (https://..., ssh://... ou git@hôte:org/depot.git)", r)
}

// Email valide une adresse e-mail (utilisée pour Let's Encrypt).
func Email(e string) error {
	if !emailRe.MatchString(e) || strings.ContainsAny(e, "'\"`$;|&<>\\") {
		return invalid("adresse e-mail %q invalide", e)
	}
	return nil
}

// Range valide qu'un entier est compris dans [min, max].
func Range(name string, v, min, max int) error {
	if v < min || v > max {
		return invalid("%s doit être compris entre %d et %d (reçu %d)", name, min, max, v)
	}
	return nil
}
