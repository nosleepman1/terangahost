package site

import (
	"strings"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/templates"
)

// ConfigData alimente les modèles de configuration d'un site.
type ConfigData struct {
	ID              string
	Domain          string
	ServerNames     string
	PHPVersion      string
	Directory       string
	SSL             bool
	Reverb          bool
	ReverbPort      int
	QueueWorkers    int
	MaxChildren     int
	StartServers    int
	MinSpareServers int
	MaxSpareServers int
}

// NewConfigData prépare les données de modèle pour un site.
func NewConfigData(s *domain.Site, ssl bool, maxChildren int) ConfigData {
	if maxChildren < 3 {
		maxChildren = 3
	}
	quarter := maxChildren / 4
	if quarter < 1 {
		quarter = 1
	}
	maxSpare := maxChildren / 2
	if maxSpare < quarter+1 {
		maxSpare = quarter + 1
	}
	return ConfigData{
		ID:              s.ID,
		Domain:          s.Domain,
		ServerNames:     strings.Join(append([]string{s.Domain}, s.Aliases...), " "),
		PHPVersion:      s.PHPVersion,
		Directory:       s.Directory,
		SSL:             ssl,
		Reverb:          s.WithReverb,
		ReverbPort:      s.ReverbPort,
		QueueWorkers:    s.QueueWorkers,
		MaxChildren:     maxChildren,
		StartServers:    quarter,
		MinSpareServers: quarter,
		MaxSpareServers: maxSpare,
	}
}

// RenderNginx génère le vhost Nginx.
func RenderNginx(d ConfigData) ([]byte, error) {
	return templates.Render("nginx/laravel_api.conf.tmpl", d)
}

// RenderFPMPool génère le pool PHP-FPM dédié.
func RenderFPMPool(d ConfigData) ([]byte, error) {
	return templates.Render("php/fpm_pool.conf.tmpl", d)
}

// RenderWorker génère le programme Supervisor des workers de queue.
func RenderWorker(d ConfigData) ([]byte, error) {
	return templates.Render("supervisor/queue_worker.conf.tmpl", d)
}

// RenderReverb génère le programme Supervisor de Laravel Reverb.
func RenderReverb(d ConfigData) ([]byte, error) {
	return templates.Render("supervisor/reverb.conf.tmpl", d)
}

// RenderScheduler génère la tâche cron du scheduler Laravel.
func RenderScheduler(d ConfigData) ([]byte, error) { return templates.Render("cron/scheduler.tmpl", d) }

// DBCredentials décrit la base de données créée pour un site.
type DBCredentials struct {
	Connection string // mysql, pgsql ou sqlite
	Port       int
	Name       string
	User       string
	Password   string
}

// EnvData alimente le modèle du fichier .env.
type EnvData struct {
	ID, AppName, AppKey, URL, Domain, Directory string
	SSL, Redis, Reverb                          bool
	ReverbPort                                  int
	ReverbAppID, ReverbKey, ReverbSecret        string
	DBConnection, DBName, DBUser, DBPassword    string
	DBPort                                      int
}

// RenderEnv génère un fichier .env de production complet (APP_KEY, base, Redis, Reverb).
func RenderEnv(s *domain.Site, srv *domain.Server, db DBCredentials, ssl bool) ([]byte, error) {
	scheme := "http"
	if ssl {
		scheme = "https"
	}
	return templates.Render("laravel/env.tmpl", EnvData{
		ID:           s.ID,
		AppName:      s.Domain,
		AppKey:       AppKey(),
		URL:          scheme + "://" + s.Domain,
		Domain:       s.Domain,
		Directory:    s.Directory,
		SSL:          ssl,
		Redis:        srv.WithRedis,
		Reverb:       s.WithReverb,
		ReverbPort:   s.ReverbPort,
		ReverbAppID:  randomHex(4),
		ReverbKey:    randomHex(10),
		ReverbSecret: randomHex(16),
		DBConnection: db.Connection,
		DBName:       db.Name,
		DBUser:       db.User,
		DBPassword:   db.Password,
		DBPort:       db.Port,
	})
}
