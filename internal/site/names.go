// Package site configure les applications Laravel sur un serveur provisionné :
// arborescence, .env et base de données, pool PHP-FPM, vhost Nginx, SSL, workers et scheduler.
package site

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// WebRoot est la racine des sites sur le serveur.
const WebRoot = "/var/www"

// ID dérive l'identifiant technique d'un site (pool, programmes supervisor, cron) de son domaine.
func ID(domainName string) string {
	var b strings.Builder
	for _, c := range domainName {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			b.WriteRune(c)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// Directory retourne le dossier racine d'un site.
func Directory(domainName string) string {
	return WebRoot + "/" + domainName
}

// DBName dérive un nom de base (et d'utilisateur) valide pour MariaDB et PostgreSQL (≤ 48 caractères).
func DBName(domainName string) string {
	id := ID(domainName)
	if len(id) <= 48 {
		return id
	}
	sum := sha256.Sum256([]byte(domainName))
	return id[:41] + "_" + hex.EncodeToString(sum[:])[:6]
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("générateur aléatoire indisponible: %v", err))
	}
	return hex.EncodeToString(b)
}

// AppKey génère une clé Laravel (équivalent de php artisan key:generate).
func AppKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("générateur aléatoire indisponible: %v", err))
	}
	return "base64:" + base64.StdEncoding.EncodeToString(b)
}

// Chemins des fichiers de configuration d'un site.
func nginxAvailable(d string) string    { return "/etc/nginx/sites-available/" + d }
func nginxEnabled(d string) string      { return "/etc/nginx/sites-enabled/" + d }
func fpmPool(v, id string) string       { return fmt.Sprintf("/etc/php/%s/fpm/pool.d/%s.conf", v, id) }
func supervisorWorker(id string) string { return "/etc/supervisor/conf.d/" + id + "-worker.conf" }
func supervisorReverb(id string) string { return "/etc/supervisor/conf.d/" + id + "-reverb.conf" }
func cronFile(id string) string         { return "/etc/cron.d/terangahost-" + id }
func certPath(d string) string          { return "/etc/letsencrypt/live/" + d + "/fullchain.pem" }
