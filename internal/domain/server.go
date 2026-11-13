package domain

import "time"

// Statuts possibles d'un serveur.
const (
	StatusProvisioning = "provisioning"
	StatusReady        = "ready"
	StatusError        = "error"
)

// DeployUser est l'utilisateur Unix non privilégié qui possède le code et exécute PHP-FPM,
// les workers de queue et le scheduler.
const DeployUser = "deployer"

// Server représente un serveur VPS géré par TerangaHost.
type Server struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	IP         string       `json:"ip"`
	SSHPort    int          `json:"ssh_port"`
	AdminUser  string       `json:"root_user"` // root ou sudoer sans mot de passe utilisé pour l'administration
	DeployUser string       `json:"deploy_user"`
	SSHKeyPath string       `json:"ssh_key_path,omitempty"`
	PHPVersion string       `json:"php_version"`
	Database   string       `json:"database"` // "mariadb", "postgres" ou "none"
	WithRedis  bool         `json:"with_redis"`
	DeployKey  string       `json:"deploy_key,omitempty"` // clé publique Git du deployer
	Hardware   HardwareSpec `json:"hardware"`
	Status     string       `json:"status"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

// Site représente une application / API Laravel hébergée sur un serveur.
type Site struct {
	ID             string    `json:"id"`
	ServerID       string    `json:"server_id"`
	Domain         string    `json:"domain"`
	Aliases        []string  `json:"aliases"`
	PHPVersion     string    `json:"php_version"`
	Directory      string    `json:"directory"`
	HasSSL         bool      `json:"has_ssl"`
	QueueWorkers   int       `json:"queue_workers"`
	WithReverb     bool      `json:"with_reverb"`
	ReverbPort     int       `json:"reverb_port,omitempty"`
	Scheduler      bool      `json:"scheduler"`
	DBName         string    `json:"db_name,omitempty"`
	Repository     string    `json:"repository,omitempty"`
	Branch         string    `json:"branch,omitempty"`
	CurrentRelease string    `json:"current_release,omitempty"`
	LastCommit     string    `json:"last_commit,omitempty"`
	LastDeployAt   time.Time `json:"last_deploy_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
