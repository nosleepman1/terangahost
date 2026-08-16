package domain

import "time"

// Server représente un serveur VPS géré par TerangaHost
type Server struct {
	ID         string       `json:"id"`          // Ex: "srv_dakar_prod_01"
	Name       string       `json:"name"`        // Ex: "dakar-prod"
	IP         string       `json:"ip"`          // Ex: "192.168.1.50"
	SSHPort    int          `json:"ssh_port"`    // Ex: 22
	RootUser   string       `json:"root_user"`   // "root" ou sudoer initial
	DeployUser string       `json:"deploy_user"` // "deployer"
	SSHKeyPath string       `json:"ssh_key_path"`// Ex: "~/.ssh/id_ed25519"
	PHPVersion string       `json:"php_version"` // "8.2", "8.3", "8.4"
	Database   string       `json:"database"`    // "mariadb", "postgres", "none"
	WithRedis  bool         `json:"with_redis"`  // true / false
	Hardware   HardwareSpec `json:"hardware"`
	Status     string       `json:"status"`      // "provisioning", "ready", "error"
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

// Site représente une application / API Laravel hébergée sur un serveur
type Site struct {
	ID          string    `json:"id"`          // Ex: "site_api_teranga"
	ServerID    string    `json:"server_id"`   // ID du serveur lié
	Domain      string    `json:"domain"`      // Ex: "api.monprojet.sn"
	Aliases     []string  `json:"aliases"`     // Ex: ["www.api.monprojet.sn"]
	PHPVersion  string    `json:"php_version"` // "8.3"
	WebRoot     string    `json:"web_root"`    // "/public"
	Directory   string    `json:"directory"`   // "/var/www/api.monprojet.sn"
	HasSSL      bool      `json:"has_ssl"`     // true / false
	QueueWorkers int      `json:"queue_workers"`// Nombre de workers supervisor
	WithReverb  bool      `json:"with_reverb"` // Support WebSockets Laravel Reverb
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
