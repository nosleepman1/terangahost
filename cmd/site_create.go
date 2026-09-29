package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/engine"
	"github.com/nosleepman1/terangahost/internal/platform/dns"
	"github.com/nosleepman1/terangahost/internal/site"
	"github.com/nosleepman1/terangahost/internal/ui"
	"github.com/nosleepman1/terangahost/internal/validate"
	"github.com/spf13/cobra"
)

var sc struct {
	server, domain, php, email, repo, branch string
	aliases                                  []string
	ssl, reverb, scheduler, noDB, skipDNS    bool
	workers, reverbPort                      int
}

var siteCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Crée ou met à jour un site Laravel (Nginx, pool PHP-FPM, .env, base, SSL, workers, scheduler)",
	Long: `Configure un site Laravel sur un serveur provisionné. La commande est idempotente : relancez-la
avec d'autres options (--workers, --reverb, --php...) pour mettre à jour la configuration.
Le fichier .env (APP_KEY, identifiants de base de données...) n'est créé qu'une seule fois.`,
	Example: `  terangahost site create --server=prod --domain=api.exemple.com --repo=git@github.com:org/api.git
  terangahost site create --server=prod --domain=app.exemple.com --alias=www.app.exemple.com --workers=3 --reverb`,
	RunE: func(*cobra.Command, []string) error { return runSiteCreate() },
}

func init() {
	f := siteCreateCmd.Flags()
	f.StringVar(&sc.server, "server", "", "Nom du serveur hôte")
	f.StringVar(&sc.domain, "domain", "", "Domaine principal (ex: api.monprojet.sn)")
	f.StringSliceVar(&sc.aliases, "alias", nil, "Domaines supplémentaires (répétable, ex: --alias=www.monprojet.sn)")
	f.StringVar(&sc.php, "php", "", "Version PHP du site (par défaut : celle du serveur)")
	f.BoolVar(&sc.ssl, "ssl", true, "Obtient un certificat Let's Encrypt (si le DNS pointe déjà vers le serveur)")
	f.StringVar(&sc.email, "email", "", "E-mail Let's Encrypt (alertes d'expiration, recommandé)")
	f.IntVar(&sc.workers, "workers", 1, "Nombre de workers de queue Supervisor (0 pour aucun)")
	f.BoolVar(&sc.reverb, "reverb", false, "Active Laravel Reverb (WebSockets)")
	f.IntVar(&sc.reverbPort, "reverb-port", 8080, "Port local de Reverb (unique par serveur)")
	f.BoolVar(&sc.scheduler, "scheduler", true, "Active le scheduler Laravel (cron chaque minute)")
	f.BoolVar(&sc.noDB, "no-db", false, "Ne crée pas de base dédiée (SQLite dans shared/)")
	f.BoolVar(&sc.skipDNS, "skip-dns-check", false, "Ignore la vérification DNS avant Let's Encrypt")
	f.StringVar(&sc.repo, "repo", "", "Dépôt Git mémorisé pour 'site deploy'")
	f.StringVar(&sc.branch, "branch", "main", "Branche Git mémorisée pour 'site deploy'")
	_ = siteCreateCmd.MarkFlagRequired("server")
	_ = siteCreateCmd.MarkFlagRequired("domain")
	siteCmd.AddCommand(siteCreateCmd)
}

func runSiteCreate() error {
	domainName, err := validate.Domain(sc.domain)
	if err != nil {
		return err
	}
	var aliases []string
	for _, a := range sc.aliases {
		alias, err := validate.Domain(a)
		if err != nil {
			return err
		}
		if alias != domainName {
			aliases = append(aliases, alias)
		}
	}
	if err := validate.Range("--workers", sc.workers, 0, 32); err != nil {
		return err
	}
	if err := validate.Range("--reverb-port", sc.reverbPort, 1024, 65535); err != nil {
		return err
	}
	if sc.email != "" {
		if err := validate.Email(sc.email); err != nil {
			return err
		}
	}
	if sc.repo != "" {
		if err := validate.Repository(sc.repo); err != nil {
			return err
		}
	}
	if err := validate.Branch(sc.branch); err != nil {
		return err
	}

	store, err := openStore()
	if err != nil {
		return err
	}
	ctx, cancel := commandContext(30 * time.Minute)
	defer cancel()

	srv, err := store.FindServer(ctx, sc.server)
	if err != nil {
		return err
	}
	phpVersion := srv.PHPVersion
	if sc.php != "" {
		phpVersion = sc.php
	}
	if err := validate.PHPVersion(phpVersion); err != nil {
		return err
	}

	s, err := store.FindSite(ctx, domainName)
	switch {
	case errors.Is(err, domain.ErrSiteNotFound):
		s = &domain.Site{ID: site.ID(domainName), Domain: domainName, Directory: site.Directory(domainName), ServerID: srv.ID, CreatedAt: time.Now()}
	case err != nil:
		return err
	case s.ServerID != srv.ID:
		return fmt.Errorf("%w (%s)", domain.ErrSiteAlreadyExists, domainName)
	default:
		ui.Info("Le site %s existe : sa configuration va être mise à jour.", domainName)
	}

	siblings, err := store.ListSites(ctx, srv.ID)
	if err != nil {
		return err
	}
	count := 1
	for _, o := range siblings {
		if o.Domain == domainName {
			continue
		}
		count++
		if sc.reverb && o.WithReverb && o.ReverbPort == sc.reverbPort {
			return fmt.Errorf("%w: le port Reverb %d est déjà utilisé par %s (--reverb-port)", domain.ErrInvalidInput, sc.reverbPort, o.Domain)
		}
	}

	s.Aliases, s.PHPVersion, s.QueueWorkers, s.WithReverb, s.ReverbPort, s.Scheduler = aliases, phpVersion, sc.workers, sc.reverb, sc.reverbPort, sc.scheduler
	if sc.repo != "" {
		s.Repository, s.Branch = sc.repo, sc.branch
	} else if s.Branch == "" {
		s.Branch = sc.branch
	}
	maxChildren := srv.Hardware.FpmMaxChildrenPerSite(count)

	log := openLog("site_" + s.ID)
	defer log.Close()
	r, err := connectAdmin(srv, log)
	if err != nil {
		return err
	}
	defer r.Close()
	m := &site.Manager{R: r, Srv: srv}

	ui.Section(fmt.Sprintf("Configuration de %s sur %s", domainName, srv.Name))
	var phpInstalled, envCreated, ssl bool
	steps := []struct {
		title string
		fn    func() error
	}{
		{"PHP " + phpVersion, func() error { phpInstalled, err = m.EnsurePHP(ctx, phpVersion); return err }},
		{"Arborescence zero-downtime (releases/, shared/)", func() error { return m.PrepareDirectories(ctx, s) }},
		{"Fichier .env et base de données", func() error {
			envCreated, err = m.EnsureEnv(ctx, s, !sc.noDB, m.CertificateExists(ctx, domainName))
			return err
		}},
		{fmt.Sprintf("Pool PHP-FPM dédié (%d processus max)", maxChildren), func() error { return m.ConfigureFPM(ctx, s, maxChildren) }},
		{"VirtualHost Nginx", func() error { ssl, err = m.ConfigureNginx(ctx, s, maxChildren); return err }},
		{fmt.Sprintf("Supervisor (%d worker(s), Reverb: %v)", s.QueueWorkers, s.WithReverb), func() error { return m.ConfigureWorkers(ctx, s) }},
		{fmt.Sprintf("Scheduler Laravel (%v)", s.Scheduler), func() error { return m.ConfigureScheduler(ctx, s) }},
	}
	for _, st := range steps {
		if err := task(st.title, st.fn); err != nil {
			return err
		}
	}
	if phpInstalled {
		ui.Info("    %s", ui.Gray("PHP "+phpVersion+" a été installé sur le serveur"))
	}

	sslErr := error(nil)
	if sc.ssl && !ssl {
		sslErr = enableSSL(ctx, m, srv, s, sc.email, sc.skipDNS)
		ssl = sslErr == nil
	}
	s.HasSSL = ssl
	if err := store.SaveSite(ctx, s); err != nil {
		return err
	}

	scheme := "http"
	if ssl {
		scheme = "https"
	}
	fmt.Println()
	ui.Rule()
	ui.Success("Site %s configuré", ui.Cyan(domainName))
	ui.KV("URL", scheme+"://"+domainName)
	ui.KV("Dossier", s.Directory)
	ui.KV("PHP", s.PHPVersion)
	if envCreated {
		db := "SQLite (shared/database.sqlite)"
		if s.DBName != "" {
			db = s.DBName + " (" + srv.Database + ")"
		}
		ui.KV("Base de données", db)
		ui.KV(".env", "généré (APP_KEY, identifiants) — modifiable avec 'site env edit'")
	}
	ui.Rule()
	if sslErr != nil {
		ui.Warn("SSL non activé : %v", sslErr)
		ui.Info("  Une fois le DNS en place :")
		ui.Hint("terangahost site ssl --domain=" + domainName)
	}
	ui.Info("\nÉtape suivante :")
	if s.Repository != "" {
		ui.Hint("terangahost site deploy --domain=" + domainName)
	} else {
		ui.Hint("terangahost site deploy --domain=" + domainName + " --repo=git@github.com:org/api.git")
	}
	if srv.DeployKey != "" {
		ui.Info("  Dépôt privé ? Ajoutez la deploy key du serveur : terangahost server deploy-key --name=%s", srv.Name)
	}
	return nil
}

// enableSSL vérifie le DNS, obtient le certificat puis bascule le vhost et le .env en HTTPS.
func enableSSL(ctx context.Context, m *site.Manager, srv *domain.Server, s *domain.Site, email string, skipDNS bool) error {
	if !skipDNS {
		ipv6 := engine.DetectIPv6(ctx, m.R)
		for _, d := range append([]string{s.Domain}, s.Aliases...) {
			if err := task("Vérification DNS de "+d, func() error { return dns.PreFlightDNSCheck(ctx, d, srv.IP, ipv6) }); err != nil {
				return err
			}
		}
	}
	if err := task("Certificat Let's Encrypt", func() error { return m.IssueCertificate(ctx, s, email) }); err != nil {
		return err
	}
	return task("Activation HTTPS (Nginx, .env)", func() error {
		ssl, err := m.ConfigureNginx(ctx, s, 0)
		if err != nil {
			return err
		}
		if !ssl {
			return fmt.Errorf("certificat introuvable après son émission")
		}
		if err := m.SwitchEnvToHTTPS(ctx, s); err != nil {
			return err
		}
		return m.RefreshApp(ctx, s)
	})
}
