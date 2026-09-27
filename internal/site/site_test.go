package site

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/tests/mocks"
)

func newSite() *domain.Site {
	return &domain.Site{
		ID: ID("api.example.com"), Domain: "api.example.com", Aliases: []string{"www.api.example.com"},
		Directory: Directory("api.example.com"), PHPVersion: "8.3", QueueWorkers: 2, Scheduler: true, ReverbPort: 8080,
	}
}

func newServer(db string) *domain.Server {
	return &domain.Server{Name: "prod", Database: db, WithRedis: true, Hardware: domain.HardwareSpec{TotalRAMMB: 2048}}
}

func TestNames(t *testing.T) {
	if ID("api.my-app.sn") != "api_my_app_sn" {
		t.Error(ID("api.my-app.sn"))
	}
	long := strings.Repeat("a", 60) + ".example.com"
	if n := DBName(long); len(n) > 48 || n == DBName(strings.Repeat("a", 61)+".example.com") {
		t.Errorf("DBName doit être court et unique: %q", n)
	}
	if !strings.HasPrefix(AppKey(), "base64:") || len(AppKey()) != 51 {
		t.Error("APP_KEY invalide")
	}
}

func TestNginxTemplate(t *testing.T) {
	s := newSite()
	httpConf, err := RenderNginx(NewConfigData(s, false, 10))
	if err != nil {
		t.Fatal(err)
	}
	h := string(httpConf)
	for _, want := range []string{
		"server_name api.example.com www.api.example.com;",
		"root /var/www/api.example.com/current/public;",
		"fastcgi_pass unix:/run/php/php8.3-fpm-api_example_com.sock;",
		"location ^~ /.well-known/acme-challenge/",
		"$realpath_root$fastcgi_script_name",
	} {
		if !strings.Contains(h, want) {
			t.Errorf("vhost HTTP: %q manquant", want)
		}
	}
	for _, unwanted := range []string{"listen 443", "location /app", "proxy_pass", "&#"} {
		if strings.Contains(h, unwanted) {
			t.Errorf("vhost HTTP ne doit pas contenir %q", unwanted)
		}
	}

	s.WithReverb = true
	sslConf, _ := RenderNginx(NewConfigData(s, true, 10))
	sc := string(sslConf)
	for _, want := range []string{"listen 443 ssl http2;", "return 301 https://$host$request_uri;", "/etc/letsencrypt/live/api.example.com/fullchain.pem", "proxy_pass http://127.0.0.1:8080;", "Strict-Transport-Security"} {
		if !strings.Contains(sc, want) {
			t.Errorf("vhost HTTPS: %q manquant", want)
		}
	}
}

func TestOtherTemplates(t *testing.T) {
	s := newSite()
	d := NewConfigData(s, false, 12)
	pool, _ := RenderFPMPool(d)
	if !strings.Contains(string(pool), "pm.max_children = 12") || !strings.Contains(string(pool), "/var/www/api.example.com/shared/storage/logs/php-fpm-error.log") {
		t.Errorf("pool FPM incorrect:\n%s", pool)
	}
	worker, _ := RenderWorker(d)
	if !strings.Contains(string(worker), "numprocs=2") || !strings.Contains(string(worker), "exec php8.3 /var/www/api.example.com/current/artisan queue:work") {
		t.Errorf("worker incorrect:\n%s", worker)
	}
	cron, _ := RenderScheduler(d)
	if !strings.Contains(string(cron), "* * * * * deployer") || !strings.HasSuffix(string(cron), "\n") {
		t.Errorf("cron incorrect:\n%q", cron)
	}
}

func TestEnvTemplate(t *testing.T) {
	s := newSite()
	s.WithReverb = true
	env, err := RenderEnv(s, newServer("mariadb"), DBCredentials{Connection: "mysql", Port: 3306, Name: "db", User: "db", Password: "secret"}, true)
	if err != nil {
		t.Fatal(err)
	}
	e := string(env)
	for _, want := range []string{"APP_ENV=production", "APP_DEBUG=false", "APP_URL=https://api.example.com", "DB_CONNECTION=mysql", "DB_PASSWORD=secret",
		"QUEUE_CONNECTION=redis", "REDIS_PREFIX=api_example_com_", "REVERB_SERVER_PORT=8080", "REVERB_SCHEME=https", "SESSION_SECURE_COOKIE=true"} {
		if !strings.Contains(e, want) {
			t.Errorf(".env: %q manquant", want)
		}
	}

	noRedis := newServer("none")
	noRedis.WithRedis = false
	env2, _ := RenderEnv(newSite(), noRedis, DBCredentials{Connection: "sqlite"}, false)
	for _, want := range []string{"DB_CONNECTION=sqlite", "DB_DATABASE=/var/www/api.example.com/shared/database.sqlite", "CACHE_STORE=file", "QUEUE_CONNECTION=database", "APP_URL=http://"} {
		if !strings.Contains(string(env2), want) {
			t.Errorf(".env sans Redis: %q manquant", want)
		}
	}
}

func TestEnsureEnvCreatesDatabaseWithoutLeakingPassword(t *testing.T) {
	r := mocks.NewMockRunner()
	m := &Manager{R: r, Srv: newServer("mariadb")}
	s := newSite()
	created, err := m.EnsureEnv(context.Background(), s, true, false)
	if err != nil || !created {
		t.Fatalf("EnsureEnv: %v created=%v", err, created)
	}
	env := string(r.Files["/var/www/api.example.com/shared/.env"])
	sql := r.InputFor("mysql --protocol=socket")
	if !strings.Contains(sql, "CREATE DATABASE IF NOT EXISTS `api_example_com`") {
		t.Errorf("SQL inattendu: %s", sql)
	}
	var password string
	for _, l := range strings.Split(env, "\n") {
		if v, ok := strings.CutPrefix(l, "DB_PASSWORD="); ok {
			password = v
		}
	}
	if len(password) != 32 || !strings.Contains(sql, password) {
		t.Fatalf("mot de passe incohérent entre .env et SQL")
	}
	for _, c := range r.Commands {
		if strings.Contains(c, password) {
			t.Fatalf("le mot de passe apparaît dans une commande: %s", c)
		}
	}
	if r.Modes["/var/www/api.example.com/shared/.env"] != 0o640 || s.DBName != "api_example_com" {
		t.Error("droits du .env ou nom de base incorrects")
	}
}

func TestEnsureEnvNeverOverwrites(t *testing.T) {
	r := mocks.NewMockRunner()
	r.Existing["/var/www/api.example.com/shared/.env"] = true
	m := &Manager{R: r, Srv: newServer("mariadb")}
	created, err := m.EnsureEnv(context.Background(), newSite(), true, false)
	if err != nil || created || len(r.Files) != 0 || r.HasExecuted("mysql") {
		t.Fatalf("un .env existant ne doit jamais être modifié (created=%v err=%v)", created, err)
	}
}

func TestConfigureNginxRollsBackOnInvalidConfig(t *testing.T) {
	r := mocks.NewMockRunner().Fail("nginx -t", errors.New("emerg"))
	m := &Manager{R: r, Srv: newServer("none")}
	if _, err := m.ConfigureNginx(context.Background(), newSite(), 5); err == nil {
		t.Fatal("erreur attendue")
	}
	if !r.HasExecuted("mv -f /etc/nginx/sites-available/api.example.com.bak") || r.HasExecuted("systemctl reload nginx") {
		t.Error("la configuration précédente doit être restaurée et Nginx non rechargé")
	}
}

func TestConfigureNginxUsesSSLWhenCertificateExists(t *testing.T) {
	r := mocks.NewMockRunner()
	r.Existing["/etc/letsencrypt/live/api.example.com/fullchain.pem"] = true
	m := &Manager{R: r, Srv: newServer("none")}
	ssl, err := m.ConfigureNginx(context.Background(), newSite(), 5)
	if err != nil || !ssl {
		t.Fatalf("ssl=%v err=%v", ssl, err)
	}
	if !strings.Contains(string(r.Files["/etc/nginx/sites-available/api.example.com"]), "listen 443") {
		t.Error("le certificat existant doit être conservé lors d'une reconfiguration")
	}
}

func TestDeleteRefusesPathsOutsideWebRoot(t *testing.T) {
	r := mocks.NewMockRunner()
	m := &Manager{R: r, Srv: newServer("none")}
	s := newSite()
	s.Directory = "/etc"
	if err := m.Delete(context.Background(), s, DeleteOptions{PurgeFiles: true}); err == nil {
		t.Fatal("la suppression hors de /var/www doit être refusée")
	}
	if r.HasExecuted("rm -rf") {
		t.Fatal("aucun rm -rf ne doit être exécuté")
	}
}

func TestAllSiteCommandsAreValidBash(t *testing.T) {
	ctx := context.Background()
	for _, db := range []string{"mariadb", "postgres", "none"} {
		r := mocks.NewMockRunner()
		m := &Manager{R: r, Srv: newServer(db)}
		s := newSite()
		s.WithReverb = true
		steps := []func() error{
			func() error { return m.PrepareDirectories(ctx, s) },
			func() error { _, err := m.EnsureEnv(ctx, s, true, false); return err },
			func() error { return m.ConfigureFPM(ctx, s, 8) },
			func() error { _, err := m.ConfigureNginx(ctx, s, 8); return err },
			func() error { return m.IssueCertificate(ctx, s, "ops@example.com") },
			func() error { return m.SwitchEnvToHTTPS(ctx, s) },
			func() error { return m.RefreshApp(ctx, s) },
			func() error { return m.ConfigureWorkers(ctx, s) },
			func() error { return m.ConfigureScheduler(ctx, s) },
			func() error { return m.Delete(ctx, s, DeleteOptions{PurgeFiles: true, DropDatabase: true}) },
		}
		for _, step := range steps {
			if err := step(); err != nil {
				t.Fatalf("%s: %v", db, err)
			}
		}
		mocks.AssertBashSyntax(t, r)
	}
}

func TestPrepareDirectoriesOwnsEveryLevel(t *testing.T) {
	r := mocks.NewMockRunner()
	m := &Manager{R: r, Srv: newServer("none")}
	if err := m.PrepareDirectories(context.Background(), newSite()); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"/shared/storage ", "/shared/storage/app ", "/shared/storage/framework ", "/shared/storage/framework/cache "} {
		if !r.HasExecuted("-o deployer -g www-data") || !r.HasExecuted(d) {
			t.Errorf("le dossier intermédiaire %q doit être créé explicitement pour appartenir au deployer", strings.TrimSpace(d))
		}
	}
}

func TestRootWrittenLogsStayOutOfDeployerDirectories(t *testing.T) {
	d := NewConfigData(newSite(), false, 8)
	d.Reverb = true
	for name, render := range map[string]func(ConfigData) ([]byte, error){"worker": RenderWorker, "reverb": RenderReverb, "pool": RenderFPMPool} {
		out, _ := render(d)
		for _, l := range strings.Split(string(out), "\n") {
			if (strings.HasPrefix(l, "stdout_logfile=") || strings.HasPrefix(l, "slowlog")) && strings.Contains(l, "/var/www/") {
				t.Errorf("%s: log écrit par root dans un dossier du deployer: %s", name, l)
			}
		}
	}
}
