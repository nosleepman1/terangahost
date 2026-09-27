package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
)

// Variables d'environnement permettant un usage non interactif (CI).
const (
	EnvPassword   = "TERANGAHOST_SSH_PASSWORD"
	EnvPassphrase = "TERANGAHOST_SSH_PASSPHRASE"
)

// ClientOptions contient les paramètres de connexion SSH.
type ClientOptions struct {
	Host           string
	Port           int
	User           string
	PrivateKeyPath string // clé explicite ; sinon agent SSH puis clés par défaut
	Password       string // authentification par mot de passe en repli
	Timeout        time.Duration
}

// Client est une connexion SSH qui mémorise la clé publique ayant réellement servi à l'authentification.
type Client struct {
	*ssh.Client
	rec *recorder
}

// AuthorizedKey retourne la clé publique utilisée pour se connecter au format authorized_keys,
// ou une chaîne vide si l'authentification s'est faite par mot de passe.
func (c *Client) AuthorizedKey() string {
	if k := c.rec.get(); k != nil {
		return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k)))
	}
	return ""
}

// Dial établit une connexion SSH vérifiée (TOFU) avec KeepAlive.
func Dial(opts ClientOptions) (*Client, error) {
	if opts.Port == 0 {
		opts.Port = 22
	}
	if opts.Timeout == 0 {
		opts.Timeout = 15 * time.Second
	}
	if opts.Password == "" {
		opts.Password = os.Getenv(EnvPassword)
	}

	signers, err := loadSigners(opts.PrivateKeyPath)
	if err != nil {
		return nil, err
	}
	rec := &recorder{}
	var auth []ssh.AuthMethod
	if len(signers) > 0 {
		wrapped := make([]ssh.Signer, len(signers))
		for i, s := range signers {
			wrapped[i] = wrapSigner(s, rec)
		}
		auth = append(auth, ssh.PublicKeys(wrapped...))
	}
	if opts.Password != "" {
		pw := opts.Password
		auth = append(auth, ssh.Password(pw), ssh.KeyboardInteractive(
			func(_, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = pw
				}
				return answers, nil
			}))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("%w: aucune clé SSH trouvée (--ssh-key, agent SSH, ~/.ssh/id_ed25519...) et aucun mot de passe fourni", domain.ErrSSHAuthentication)
	}

	tofu, err := NewTOFUCallback()
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            auth,
		HostKeyCallback: tofu.Callback(),
		Timeout:         opts.Timeout,
	}

	address := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	conn, err := net.DialTimeout("tcp", address, opts.Timeout)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrSSHConnectionTimeout, err)
	}
	// Délai maximal pour la poignée de main SSH, puis on repasse en mode sans échéance.
	_ = conn.SetDeadline(time.Now().Add(opts.Timeout))
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		conn.Close()
		if errors.Is(err, domain.ErrHostKeyMismatch) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s@%s: %v", domain.ErrSSHAuthentication, opts.User, address, err)
	}
	_ = conn.SetDeadline(time.Time{})

	client := ssh.NewClient(sshConn, chans, reqs)
	go keepAlive(client, 15*time.Second)
	return &Client{Client: client, rec: rec}, nil
}

func keepAlive(client *ssh.Client, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
			return
		}
	}
}

// loadSigners retourne les clés à proposer : la clé explicite (erreur si illisible),
// sinon les clés de l'agent SSH puis les clés standard de ~/.ssh.
func loadSigners(explicit string) ([]ssh.Signer, error) {
	if explicit != "" {
		s, err := parseKeyFile(ExpandHome(explicit), true)
		if err != nil {
			return nil, fmt.Errorf("%w: clé %s: %v", domain.ErrSSHAuthentication, explicit, err)
		}
		return []ssh.Signer{s}, nil
	}

	var signers []ssh.Signer
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			if s, err := agent.NewClient(conn).Signers(); err == nil {
				signers = append(signers, s...)
			}
		}
	}
	home, _ := os.UserHomeDir()
	for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		path := filepath.Join(home, ".ssh", name)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if s, err := parseKeyFile(path, false); err == nil {
			signers = append(signers, s)
		}
	}
	return signers, nil
}

// parseKeyFile lit une clé privée, en demandant la passphrase si nécessaire.
func parseKeyFile(path string, promptAllowed bool) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(data)
	var missing *ssh.PassphraseMissingError
	if !errors.As(err, &missing) {
		return signer, err
	}
	passphrase := os.Getenv(EnvPassphrase)
	if passphrase == "" {
		if !promptAllowed || !term.IsTerminal(int(os.Stdin.Fd())) {
			return nil, fmt.Errorf("clé protégée par une passphrase (ajoutez-la à ssh-agent ou définissez %s)", EnvPassphrase)
		}
		p, err := PromptSecret(fmt.Sprintf("Passphrase de la clé %s : ", path))
		if err != nil {
			return nil, err
		}
		passphrase = p
	}
	return ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
}

// PromptSecret lit une valeur secrète sur le terminal sans l'afficher.
func PromptSecret(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return string(b), err
}

// ExpandHome remplace le préfixe ~/ par le dossier personnel.
func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// recorder mémorise la clé dont la signature a été produite, c'est-à-dire celle acceptée par le serveur.
type recorder struct {
	mu  sync.Mutex
	key ssh.PublicKey
}

func (r *recorder) set(k ssh.PublicKey) { r.mu.Lock(); r.key = k; r.mu.Unlock() }
func (r *recorder) get() ssh.PublicKey  { r.mu.Lock(); defer r.mu.Unlock(); return r.key }

// wrapSigner enveloppe un signer en conservant ses capacités (algorithmes rsa-sha2-*).
func wrapSigner(s ssh.Signer, rec *recorder) ssh.Signer {
	if ms, ok := s.(ssh.MultiAlgorithmSigner); ok {
		return &multiAlgRecorder{algRecorder: &algRecorder{AlgorithmSigner: ms, rec: rec}, m: ms}
	}
	if as, ok := s.(ssh.AlgorithmSigner); ok {
		return &algRecorder{AlgorithmSigner: as, rec: rec}
	}
	return &plainRecorder{Signer: s, rec: rec}
}

type plainRecorder struct {
	ssh.Signer
	rec *recorder
}

func (p *plainRecorder) Sign(r io.Reader, data []byte) (*ssh.Signature, error) {
	sig, err := p.Signer.Sign(r, data)
	if err == nil {
		p.rec.set(p.PublicKey())
	}
	return sig, err
}

type algRecorder struct {
	ssh.AlgorithmSigner
	rec *recorder
}

func (a *algRecorder) Sign(r io.Reader, data []byte) (*ssh.Signature, error) {
	sig, err := a.AlgorithmSigner.Sign(r, data)
	if err == nil {
		a.rec.set(a.PublicKey())
	}
	return sig, err
}

func (a *algRecorder) SignWithAlgorithm(r io.Reader, data []byte, algorithm string) (*ssh.Signature, error) {
	sig, err := a.AlgorithmSigner.SignWithAlgorithm(r, data, algorithm)
	if err == nil {
		a.rec.set(a.PublicKey())
	}
	return sig, err
}

type multiAlgRecorder struct {
	*algRecorder
	m ssh.MultiAlgorithmSigner
}

func (m *multiAlgRecorder) Algorithms() []string { return m.m.Algorithms() }
