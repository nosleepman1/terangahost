package ssh

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/teranga-host/terangahost/internal/domain"
	"golang.org/x/crypto/ssh"
)

// ClientOptions contient les paramètres de connexion SSH
type ClientOptions struct {
	Host           string
	Port           int
	User           string
	PrivateKeyPath string
	Password       string
	Timeout        time.Duration
}

// NewNativeSSHClient établit une connexion SSH sécurisée et résiliente avec KeepAlive
func NewNativeSSHClient(opts ClientOptions) (*ssh.Client, error) {
	if opts.Port == 0 {
		opts.Port = 22
	}
	if opts.Timeout == 0 {
		opts.Timeout = 15 * time.Second
	}

	var authMethods []ssh.AuthMethod

	// 1. Clé privée prioritaire
	if opts.PrivateKeyPath != "" {
		expandedPath := expandHome(opts.PrivateKeyPath)
		keyBytes, err := os.ReadFile(expandedPath)
		if err == nil {
			signer, err := ssh.ParsePrivateKey(keyBytes)
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}

	// 2. Si aucune clé fournie ou si échec, tester les clés standard par défaut
	if len(authMethods) == 0 {
		home, _ := os.UserHomeDir()
		defaultKeys := []string{
			filepathJoin(home, ".ssh", "id_ed25519"),
			filepathJoin(home, ".ssh", "id_rsa"),
		}
		for _, k := range defaultKeys {
			if data, err := os.ReadFile(k); err == nil {
				if signer, err := ssh.ParsePrivateKey(data); err == nil {
					authMethods = append(authMethods, ssh.PublicKeys(signer))
					break
				}
			}
		}
	}

	// 3. Mot de passe en repli
	if opts.Password != "" {
		authMethods = append(authMethods, ssh.Password(opts.Password))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("%w: aucune méthode d'authentification disponible (clé ou mot de passe requis)", domain.ErrSSHAuthentication)
	}

	tofu, err := NewTOFUCallback()
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            authMethods,
		HostKeyCallback: tofu.Callback(),
		Timeout:         opts.Timeout,
	}

	address := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	conn, err := net.DialTimeout("tcp", address, opts.Timeout)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrSSHConnectionTimeout, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("%w: %v", domain.ErrSSHAuthentication, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)

	// Lancer un KeepAlive en arrière-plan pour éviter les déconnexions sur les longs setups
	go startKeepAlive(client, 15*time.Second)

	return client, nil
}

func startKeepAlive(client *ssh.Client, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		_, _, err := client.SendRequest("keepalive@terangahost.org", true, nil)
		if err != nil {
			return
		}
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepathJoin(home, path[2:])
		}
	}
	return path
}

func filepathJoin(elem ...string) string {
	return strings.Join(elem, string(os.PathSeparator))
}
