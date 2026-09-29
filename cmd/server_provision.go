package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/engine"
	"github.com/nosleepman1/terangahost/internal/engine/steps"
	"github.com/nosleepman1/terangahost/internal/platform/ssh"
	"github.com/nosleepman1/terangahost/internal/ui"
	"github.com/nosleepman1/terangahost/internal/validate"
	"github.com/spf13/cobra"
)

var prov struct {
	name, ip, user, key, password, php, db string
	port                                   int
	askPassword, redis, hardenSSH          bool
}

var serverProvisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Provisionne un VPS Ubuntu 22.04/24.04 pour Laravel (idempotent : peut être relancé)",
	Long: `Configure un VPS Ubuntu 22.04/24.04 : swap, utilisateur deployer, UFW, Fail2ban, mises à jour de
sécurité automatiques, PHP-FPM, Nginx, Composer, Supervisor, Certbot, MariaDB/PostgreSQL, Redis et
durcissement SSH. La commande est idempotente : relancez-la pour réparer ou mettre à jour un serveur.`,
	Example: `  terangahost server provision --name=prod --ip=203.0.113.10 --ssh-key=~/.ssh/id_ed25519
  terangahost server provision --name=prod --ip=203.0.113.10 --user=ubuntu --db=postgres --php=8.4`,
	RunE: func(cmd *cobra.Command, _ []string) error { return runProvision() },
}

func init() {
	f := serverProvisionCmd.Flags()
	f.StringVar(&prov.name, "name", "", "Nom unique du serveur (ex: dakar-prod)")
	f.StringVar(&prov.ip, "ip", "", "Adresse IP publique du VPS")
	f.IntVar(&prov.port, "port", 22, "Port SSH")
	f.StringVar(&prov.user, "user", "root", "Utilisateur SSH initial (root ou sudoer sans mot de passe)")
	f.StringVar(&prov.key, "ssh-key", "", "Clé privée SSH (par défaut : ssh-agent puis ~/.ssh/id_ed25519, id_ecdsa, id_rsa)")
	f.BoolVar(&prov.askPassword, "ask-password", false, "Demande le mot de passe SSH de façon masquée (ou "+ssh.EnvPassword+")")
	f.StringVar(&prov.password, "password", "", "Obsolète : visible dans l'historique du shell, utilisez --ask-password")
	f.StringVar(&prov.php, "php", "8.3", "Version PHP ("+strings.Join(validate.SupportedPHPVersions, ", ")+")")
	f.StringVar(&prov.db, "db", "mariadb", "Base de données : mariadb, postgres ou none")
	f.BoolVar(&prov.redis, "redis", true, "Installe Redis (cache, sessions, queues)")
	f.BoolVar(&prov.hardenSSH, "harden-ssh", true, "Désactive l'authentification SSH par mot de passe (uniquement si vous êtes connecté par clé)")
	_ = f.MarkDeprecated("password", "utilisez --ask-password ou la variable "+ssh.EnvPassword)
	_ = serverProvisionCmd.MarkFlagRequired("name")
	_ = serverProvisionCmd.MarkFlagRequired("ip")
	serverCmd.AddCommand(serverProvisionCmd)
}

func runProvision() error {
	if err := validate.ServerName(prov.name); err != nil {
		return err
	}
	if err := validate.Host(prov.ip); err != nil {
		return err
	}
	if err := validate.Port(prov.port); err != nil {
		return err
	}
	if err := validate.PHPVersion(prov.php); err != nil {
		return err
	}
	db, err := validate.Database(prov.db)
	if err != nil {
		return err
	}

	store, err := openStore()
	if err != nil {
		return err
	}
	ctx, cancel := commandContext(90 * time.Minute)
	defer cancel()

	srv, err := store.FindServer(ctx, prov.name)
	switch {
	case errors.Is(err, domain.ErrServerNotFound):
		srv = &domain.Server{ID: fmt.Sprintf("srv_%d", time.Now().UnixNano()), Name: prov.name, CreatedAt: time.Now()}
	case err != nil:
		return err
	case srv.IP != prov.ip:
		return fmt.Errorf("%w : %q pointe vers %s (supprimez-le d'abord avec 'terangahost server remove --name=%s')",
			domain.ErrServerAlreadyExists, prov.name, srv.IP, prov.name)
	}

	password := prov.password
	if prov.askPassword {
		if password, err = ssh.PromptSecret(fmt.Sprintf("Mot de passe SSH de %s@%s : ", prov.user, prov.ip)); err != nil {
			return err
		}
	}

	ui.PrintBanner(Version)
	log := openLog("provision_" + prov.name)
	defer log.Close()

	var client *ssh.Client
	if err := task(fmt.Sprintf("Connexion SSH à %s@%s:%d", prov.user, prov.ip, prov.port), func() error {
		client, err = ssh.Dial(ssh.ClientOptions{
			Host: prov.ip, Port: prov.port, User: prov.user,
			PrivateKeyPath: prov.key, Password: password, Timeout: 20 * time.Second,
		})
		return err
	}); err != nil {
		return err
	}
	runner := ssh.NewRunner(client, ssh.RunnerOptions{Sudo: prov.user != "root", Log: log.Writer()})
	defer runner.Close()

	spec, err := engine.DetectHardware(ctx, runner)
	if err != nil {
		ui.Warn("%v : valeurs prudentes utilisées (1 Go de RAM)", err)
		spec.TotalRAMMB, spec.CPUCores = 1024, 1
	} else {
		ui.Info("  %s %s · %d Mo RAM · %d vCPU · %d Go libres", ui.Gray("Matériel :"), spec.OSVersion, spec.TotalRAMMB, spec.CPUCores, spec.DiskFreeGB)
	}

	srv.IP, srv.SSHPort, srv.AdminUser, srv.DeployUser = prov.ip, prov.port, prov.user, domain.DeployUser
	if prov.key != "" {
		srv.SSHKeyPath = prov.key
	}
	srv.PHPVersion, srv.Database, srv.WithRedis, srv.Hardware = prov.php, db, prov.redis, spec
	srv.Status = domain.StatusProvisioning
	if err := store.SaveServer(ctx, srv); err != nil {
		return err
	}

	authorizedKey := client.AuthorizedKey()
	harden := prov.hardenSSH && authorizedKey != ""
	if prov.hardenSSH && !harden {
		ui.Warn("connexion par mot de passe : le durcissement SSH est ignoré pour ne pas vous bloquer l'accès")
	}

	ui.Section("Provisionnement")
	pipeline := engine.NewPipeline(&engine.ConsoleListener{}, loggerOf(log))
	pipeline.AddStep(
		&steps.StepHandshake{},
		&steps.StepSwap{},
		&steps.StepSecurity{AuthorizedKey: authorizedKey},
		&steps.StepSudoers{},
		&steps.StepPHP{Version: prov.php},
		&steps.StepWebServer{},
		&steps.StepTools{},
		&steps.StepDatabase{},
		&steps.StepRedis{},
		&steps.StepSSHHardening{Enabled: harden},
	)
	start := time.Now()
	if err := pipeline.Execute(ctx, runner, srv); err != nil {
		srv.Status = domain.StatusError
		_ = store.SaveServer(ctx, srv)
		if log != nil {
			ui.Info("\nJournal détaillé : %s", log.Path)
		}
		return fmt.Errorf("provisionnement interrompu (relancez la commande pour reprendre) : %w", err)
	}
	srv.Status = domain.StatusReady
	if err := store.SaveServer(ctx, srv); err != nil {
		return err
	}

	deployerOK := false
	if authorizedKey != "" {
		if dr, err := connectDeployer(srv, nil); err == nil {
			_, err = dr.RunSilent(ctx, "true")
			deployerOK = err == nil
			dr.Close()
		}
	}

	fmt.Println()
	ui.Rule()
	ui.Success("Serveur %s provisionné en %s", ui.Cyan(srv.Name), time.Since(start).Round(time.Second))
	ui.KV("PHP", srv.PHPVersion)
	ui.KV("Base de données", srv.Database)
	ui.KV("Redis", fmt.Sprint(srv.WithRedis))
	ui.KV("SSH durci", fmt.Sprint(harden))
	if deployerOK {
		ui.KV("Accès deployer", ui.Green("OK"))
	} else {
		ui.KV("Accès deployer", ui.Yellow("non vérifié"))
		ui.Warn("ajoutez votre clé publique à /home/deployer/.ssh/authorized_keys pour pouvoir déployer")
	}
	if log != nil {
		ui.KV("Journal", log.Path)
	}
	ui.Rule()
	if srv.DeployKey != "" {
		ui.Info("\nDeploy key (à ajouter en lecture seule sur GitHub/GitLab pour les dépôts privés) :\n  %s", srv.DeployKey)
	}
	ui.Info("\nÉtape suivante :")
	ui.Hint(fmt.Sprintf("terangahost site create --server=%s --domain=api.exemple.com --repo=git@github.com:org/api.git", srv.Name))
	return nil
}
