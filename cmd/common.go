package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/platform/logger"
	"github.com/nosleepman1/terangahost/internal/platform/ssh"
	"github.com/nosleepman1/terangahost/internal/platform/storage"
	"github.com/nosleepman1/terangahost/internal/ui"
	"golang.org/x/term"
)

// commandContext crée un contexte annulé par Ctrl+C / SIGTERM ou à l'expiration du délai.
func commandContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if timeout <= 0 {
		return ctx, stop
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	return tctx, func() { cancel(); stop() }
}

func openStore() (*storage.JSONRepository, error) {
	return storage.NewJSONRepository()
}

// openLog crée le journal détaillé ; son absence n'est pas bloquante.
func openLog(prefix string) *logger.FileLogger {
	l, err := logger.NewFileLogger(prefix)
	if err != nil {
		ui.Warn("journal détaillé indisponible: %v", err)
		return nil
	}
	return l
}

// connect ouvre une connexion SSH vers le serveur sous l'utilisateur donné.
func connect(srv *domain.Server, user string, sudo bool, log *logger.FileLogger) (*ssh.Runner, error) {
	client, err := ssh.Dial(ssh.ClientOptions{
		Host:           srv.IP,
		Port:           srv.SSHPort,
		User:           user,
		PrivateKeyPath: srv.SSHKeyPath,
		Timeout:        20 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	var logW io.Writer
	if log != nil {
		logW = log.Writer()
	}
	return ssh.NewRunner(client, ssh.RunnerOptions{Sudo: sudo, Log: logW}), nil
}

// connectAdmin se connecte avec l'utilisateur d'administration (root ou sudoer) : commandes exécutées en root.
func connectAdmin(srv *domain.Server, log *logger.FileLogger) (*ssh.Runner, error) {
	return connect(srv, srv.AdminUser, srv.AdminUser != "root", log)
}

// connectDeployer se connecte avec l'utilisateur non privilégié qui possède les applications.
func connectDeployer(srv *domain.Server, log *logger.FileLogger) (*ssh.Runner, error) {
	user := srv.DeployUser
	if user == "" {
		user = domain.DeployUser
	}
	return connect(srv, user, false, log)
}

// siteWithServer charge un site et son serveur ; --server, s'il est fourni, doit correspondre.
func siteWithServer(ctx context.Context, store domain.Store, domainName, serverName string) (*domain.Site, *domain.Server, error) {
	s, err := store.FindSite(ctx, domainName)
	if err != nil {
		return nil, nil, fmt.Errorf("%w (créez-le avec 'terangahost site create')", err)
	}
	srv, err := store.FindServerByID(ctx, s.ServerID)
	if err != nil {
		return nil, nil, err
	}
	if serverName != "" && srv.Name != serverName {
		return nil, nil, fmt.Errorf("%w: %s est hébergé sur %q, pas sur %q", domain.ErrInvalidInput, domainName, srv.Name, serverName)
	}
	return s, srv, nil
}

// confirm demande une confirmation interactive, sauf si --yes est fourni.
func confirm(question string, yes bool) error {
	if yes {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("confirmation requise : relancez avec --yes")
	}
	fmt.Printf("%s [o/N] ", question)
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "o", "oui", "y", "yes":
		return nil
	}
	return fmt.Errorf("opération annulée")
}

// task exécute fn en affichant un spinner puis le résultat.
func task(title string, fn func() error) error {
	sp := ui.StartSpinner(title)
	if err := fn(); err != nil {
		sp.Fail(title)
		return err
	}
	sp.Succeed(title)
	return nil
}
