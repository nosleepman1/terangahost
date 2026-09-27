package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/shell"
	"github.com/nosleepman1/terangahost/templates"
)

// StepSecurity installe les paquets de base, crée l'utilisateur deployer (clé de connexion et
// deploy key Git), configure UFW, Fail2ban et les mises à jour de sécurité automatiques.
type StepSecurity struct {
	// AuthorizedKey est la clé publique locale à autoriser pour le deployer (peut être vide).
	AuthorizedKey string
}

func (s *StepSecurity) ID() string { return "02_security" }

func (s *StepSecurity) Title() string {
	return "Sécurisation (utilisateur deployer, UFW, Fail2ban, mises à jour auto)"
}

// PreCheck : toujours exécutée pour garantir l'état de sécurité (commandes idempotentes).
func (s *StepSecurity) PreCheck(context.Context, domain.Runner, *domain.Server) (bool, error) {
	return false, nil
}

const autoUpgrades = `APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
`

func (s *StepSecurity) Execute(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	const home = "/home/" + domain.DeployUser
	const auth = home + "/.ssh/authorized_keys"

	if err := runAll(ctx, r, "installation des paquets de base",
		aptUpdate(),
		aptInstall("ufw", "fail2ban", "python3-systemd", "curl", "git", "unzip", "zip", "acl", "software-properties-common",
			"ca-certificates", "gnupg", "lsb-release", "htop", "psmisc", "unattended-upgrades", "cron"),
	); err != nil {
		return err
	}

	if err := runAll(ctx, r, "création de l'utilisateur deployer",
		"id -u deployer >/dev/null 2>&1 || useradd -m -s /bin/bash -g www-data deployer",
		"install -d -m 700 -o deployer -g www-data "+home+"/.ssh",
		"touch "+auth,
		fmt.Sprintf(`h=$(getent passwd %s | cut -d: -f6); if [ -n "$h" ] && [ -f "$h/.ssh/authorized_keys" ] && [ "$h" != %s ]; then cat "$h/.ssh/authorized_keys" >> %s; fi`,
			shell.Quote(srv.AdminUser), home, auth),
	); err != nil {
		return err
	}
	if key := strings.TrimSpace(s.AuthorizedKey); key != "" {
		if _, err := r.RunWithInput(ctx, "cat >> "+auth, []byte("\n"+key+"\n")); err != nil {
			return fmt.Errorf("ajout de la clé SSH du deployer: %w", err)
		}
	}
	if err := runAll(ctx, r, "clés SSH du deployer",
		fmt.Sprintf("awk 'NF && !seen[$0]++' %[1]s > %[1]s.tmp && mv -f %[1]s.tmp %[1]s", auth),
		"chmod 600 "+auth,
		"[ -f "+home+"/.ssh/id_ed25519 ] || runuser -u deployer -- ssh-keygen -q -t ed25519 -N '' -C "+shell.Quote("deployer@"+srv.Name)+" -f "+home+"/.ssh/id_ed25519",
		"[ -f "+home+"/.ssh/config ] || printf 'Host *\\n    StrictHostKeyChecking accept-new\\n' > "+home+"/.ssh/config",
		"chmod 600 "+home+"/.ssh/config",
		"chown -R deployer:www-data "+home+"/.ssh",
	); err != nil {
		return err
	}
	if key, err := r.RunSilent(ctx, "cat "+home+"/.ssh/id_ed25519.pub"); err == nil {
		srv.DeployKey = strings.TrimSpace(key)
	}

	port := srv.SSHPort
	if port == 0 {
		port = 22
	}
	if err := runAll(ctx, r, "pare-feu UFW",
		"ufw default deny incoming",
		"ufw default allow outgoing",
		fmt.Sprintf("ufw allow %d/tcp", port),
		"ufw allow 80/tcp",
		"ufw allow 443/tcp",
		"ufw --force enable",
	); err != nil {
		return err
	}

	jail, err := templates.Render("fail2ban/jail.conf.tmpl", map[string]any{"SSHPort": port})
	if err != nil {
		return err
	}
	if err := r.Upload(ctx, jail, "/etc/fail2ban/jail.d/terangahost.conf", 0o644); err != nil {
		return err
	}
	if err := r.Upload(ctx, []byte(autoUpgrades), "/etc/apt/apt.conf.d/20auto-upgrades", 0o644); err != nil {
		return err
	}
	return runAll(ctx, r, "Fail2ban",
		"systemctl enable fail2ban >/dev/null 2>&1 || true",
		"systemctl restart fail2ban",
	)
}
