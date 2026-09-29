package tests

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/engine"
	"github.com/nosleepman1/terangahost/internal/engine/steps"
	"github.com/nosleepman1/terangahost/tests/mocks"
)

const ubuntu2404 = "NAME=\"Ubuntu\"\nID=ubuntu\nVERSION_ID=\"24.04\"\nPRETTY_NAME=\"Ubuntu 24.04 LTS\""

func freshServer() *domain.Server {
	return &domain.Server{
		ID: "srv_test", Name: "test-vps", IP: "203.0.113.10", SSHPort: 22,
		AdminUser: "root", DeployUser: "deployer", PHPVersion: "8.3",
		Database: "mariadb", WithRedis: true,
		Hardware: domain.HardwareSpec{TotalRAMMB: 2048, CPUCores: 2},
	}
}

func freshUbuntu() *mocks.MockRunner {
	return mocks.NewMockRunner().
		On("id -u", "0").
		On("cat /etc/os-release", ubuntu2404).
		On("echo LOCKED || echo FREE", "FREE").
		On("df -BM", "20000").
		On("id_ed25519.pub", "ssh-ed25519 AAAAdeploy deployer@test-vps").
		On("Include[[:space:]]", "READY")
}

func fullPipeline(harden bool) *engine.Pipeline {
	p := engine.NewPipeline(nil, nil)
	p.AddStep(
		&steps.StepHandshake{Interval: time.Millisecond},
		&steps.StepSwap{},
		&steps.StepSecurity{AuthorizedKey: "ssh-ed25519 AAAAlocal user@laptop"},
		&steps.StepSudoers{},
		&steps.StepPHP{Version: "8.3"},
		&steps.StepWebServer{},
		&steps.StepTools{},
		&steps.StepDatabase{},
		&steps.StepRedis{},
		&steps.StepSSHHardening{Enabled: harden},
	)
	return p
}

func TestProvisionFreshServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r := freshUbuntu()
	srv := freshServer()

	if err := fullPipeline(true).Execute(ctx, r, srv); err != nil {
		t.Fatalf("échec du pipeline: %v", err)
	}

	for _, cmd := range []string{
		"mkswap /swapfile",
		"ufw --force enable",
		"useradd -m -s /bin/bash -g www-data deployer",
		"ppa:ondrej/php",
		"php8.3-fpm",
		"php8.3-bcmath",
		"nginx -t",
		"composer-setup.php",
		"mariadb-server",
		"redis-server",
		"visudo -cf /etc/sudoers.d/terangahost-deployer.new",
		"sshd -t",
	} {
		if !r.HasExecuted(cmd) {
			t.Errorf("commande attendue non exécutée: %q", cmd)
		}
	}
	if srv.DeployKey != "ssh-ed25519 AAAAdeploy deployer@test-vps" {
		t.Errorf("deploy key non récupérée: %q", srv.DeployKey)
	}
	if !strings.Contains(r.InputFor("cat >> /home/deployer/.ssh/authorized_keys"), "AAAAlocal") {
		t.Error("la clé SSH locale doit être autorisée pour le deployer")
	}

	sudoers := string(r.Files["/etc/sudoers.d/terangahost-deployer.new"])
	if strings.Contains(sudoers, "certbot") || strings.Contains(sudoers, "*") {
		t.Errorf("la règle sudoers ne doit contenir ni certbot ni joker:\n%s", sudoers)
	}
	if tune := string(r.Files["/etc/mysql/mariadb.conf.d/99-terangahost.cnf"]); !strings.Contains(tune, "innodb_buffer_pool_size = 256M") {
		t.Errorf("réglage MariaDB incorrect:\n%s", tune)
	}
	if lr := string(r.Files["/etc/logrotate.d/terangahost"]); !strings.Contains(lr, "su deployer www-data") || strings.Contains(lr, "/var/log/nginx") {
		t.Errorf("configuration logrotate incorrecte:\n%s", lr)
	}
	if ssh := string(r.Files["/etc/ssh/sshd_config.d/00-terangahost.conf"]); !strings.Contains(ssh, "PasswordAuthentication no") {
		t.Error("le durcissement SSH doit désactiver les mots de passe")
	}
	mocks.AssertBashSyntax(t, r)
}

func TestRedisInstalledWithoutDatabase(t *testing.T) {
	r := freshUbuntu()
	srv := freshServer()
	srv.Database = "none"
	if err := fullPipeline(false).Execute(context.Background(), r, srv); err != nil {
		t.Fatal(err)
	}
	if !r.HasExecuted("redis-server") {
		t.Error("Redis doit être installé même avec --db=none")
	}
	if r.HasExecuted("mariadb-server") || r.HasExecuted("postgresql") {
		t.Error("aucune base ne doit être installée avec --db=none")
	}
	if r.HasExecuted("sshd -t") {
		t.Error("le durcissement SSH désactivé ne doit rien modifier")
	}

	pg := freshUbuntu()
	pgSrv := freshServer()
	pgSrv.Database = "postgres"
	if err := fullPipeline(false).Execute(context.Background(), pg, pgSrv); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pg.InputFor("psql"), "shared_buffers = '512MB'") {
		t.Error("réglage PostgreSQL attendu (25 % de la RAM)")
	}
	mocks.AssertBashSyntax(t, pg)
}

func TestHandshakeRejectsNonRootAndOtherOS(t *testing.T) {
	ctx := context.Background()
	step := &steps.StepHandshake{Interval: time.Millisecond, Retries: 2}

	noSudo := freshUbuntu().On("id -u", "1000")
	if err := step.Execute(ctx, noSudo, freshServer()); !errors.Is(err, domain.ErrSudoRequired) {
		t.Errorf("attendu ErrSudoRequired, obtenu %v", err)
	}

	debian := freshUbuntu().On("cat /etc/os-release", "ID=debian\nVERSION_ID=\"12\"")
	if err := step.Execute(ctx, debian, freshServer()); !errors.Is(err, domain.ErrUnsupportedOS) {
		t.Errorf("attendu ErrUnsupportedOS, obtenu %v", err)
	}

	old := freshUbuntu().On("cat /etc/os-release", "ID=ubuntu\nVERSION_ID=\"20.04\"")
	if err := step.Execute(ctx, old, freshServer()); !errors.Is(err, domain.ErrUnsupportedOS) {
		t.Errorf("Ubuntu 20.04 doit être refusé, obtenu %v", err)
	}

	locked := freshUbuntu().On("echo LOCKED || echo FREE", "LOCKED")
	if err := step.Execute(ctx, locked, freshServer()); !errors.Is(err, domain.ErrAptLockTimeout) {
		t.Errorf("attendu ErrAptLockTimeout, obtenu %v", err)
	}
}

func TestPipelineStopsOnFailureAndSkipsSatisfiedSteps(t *testing.T) {
	r := freshUbuntu().
		On("swapon --show=SIZE", "2147483648").
		On("cat /etc/sysctl.d/99-terangahost.conf", "# Managed by TerangaHost\nvm.swappiness=10\nvm.vfs_cache_pressure=50").
		Fail("ufw --force enable", errors.New("ufw indisponible"))

	err := fullPipeline(false).Execute(context.Background(), r, freshServer())
	if err == nil || !strings.Contains(err.Error(), "02_security") {
		t.Fatalf("attendu un échec à l'étape 02_security, obtenu %v", err)
	}
	if r.HasExecuted("mkswap") {
		t.Error("l'étape swap déjà satisfaite ne doit pas être rejouée")
	}
	if r.HasExecuted("ppa:ondrej/php") {
		t.Error("le pipeline doit s'arrêter à la première erreur")
	}
}

func TestSudoersRollbackOnInvalidSyntax(t *testing.T) {
	r := freshUbuntu().Fail("visudo -cf", errors.New("syntax error"))
	err := (&steps.StepSudoers{}).Execute(context.Background(), r, freshServer())
	if err == nil {
		t.Fatal("erreur attendue")
	}
	if !r.HasExecuted("rm -f /etc/sudoers.d/terangahost-deployer.new") || r.HasExecuted("mv -f /etc/sudoers.d/terangahost-deployer.new") {
		t.Error("une règle sudoers invalide ne doit jamais être activée")
	}
}
