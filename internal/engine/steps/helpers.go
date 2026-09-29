// Package steps contient les étapes idempotentes de provisionnement d'un serveur Ubuntu.
package steps

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/shell"
)

// aptOpts attend la libération du verrou dpkg au lieu d'échouer, et conserve les fichiers de
// configuration existants sans poser de question.
const aptOpts = "-o DPkg::Lock::Timeout=600 -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold"

func aptUpdate() string {
	return "apt-get update -q -o DPkg::Lock::Timeout=600"
}

func aptInstall(pkgs ...string) string {
	return "apt-get install -y -q " + aptOpts + " " + shell.Join(pkgs...)
}

// runAll exécute les commandes dans l'ordre et s'arrête à la première erreur.
func runAll(ctx context.Context, r domain.Runner, what string, cmds ...string) error {
	for _, c := range cmds {
		if _, err := r.RunSilent(ctx, c); err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
	}
	return nil
}

// packagesInstalled indique si tous les paquets sont installés.
func packagesInstalled(ctx context.Context, r domain.Runner, pkgs ...string) bool {
	out, err := r.RunSilent(ctx, "dpkg-query -W -f='${Status}\\n' "+shell.Join(pkgs...)+" 2>/dev/null | grep -c 'install ok installed'")
	if err != nil {
		return false
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	return err == nil && n == len(pkgs)
}

// hasContent indique si le fichier distant a exactement le contenu attendu.
func hasContent(ctx context.Context, r domain.Runner, path string, want []byte) bool {
	out, err := r.RunSilent(ctx, "cat "+shell.Quote(path)+" 2>/dev/null")
	return err == nil && strings.TrimSpace(out) == strings.TrimSpace(string(want))
}

// ready exécute une commande de vérification qui affiche READY si tout est en place.
func ready(ctx context.Context, r domain.Runner, check string) bool {
	out, err := r.RunSilent(ctx, "("+check+") >/dev/null 2>&1 && echo READY")
	return err == nil && strings.Contains(out, "READY")
}
