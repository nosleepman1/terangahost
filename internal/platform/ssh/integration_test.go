package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
	"golang.org/x/crypto/ssh"
)

// testServer est un serveur SSH minimal qui exécute les commandes reçues avec bash local.
type testServer struct {
	addr   string
	config *ssh.ServerConfig
}

func startTestServer(t *testing.T, authorized ssh.PublicKey) *testServer {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash indisponible")
	}
	_, hostPriv, _ := ed25519.GenerateKey(rand.Reader)
	hostSigner, _ := ssh.NewSignerFromKey(hostPriv)
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) == string(authorized.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("clé refusée")
		},
	}
	cfg.AddHostKey(hostSigner)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serve(conn, cfg)
		}
	}()
	return &testServer{addr: ln.Addr().String(), config: cfg}
}

func serve(conn net.Conn, cfg *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() != "session" {
			_ = nc.Reject(ssh.UnknownChannelType, "")
			continue
		}
		ch, requests, err := nc.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer ch.Close()
			var cmd *exec.Cmd
			for req := range requests {
				switch req.Type {
				case "exec":
					n := binary.BigEndian.Uint32(req.Payload[:4])
					cmd = exec.Command("sh", "-c", string(req.Payload[4:4+n]))
					cmd.Stdout, cmd.Stderr = ch, ch.Stderr()
					stdin, _ := cmd.StdinPipe()
					_ = req.Reply(true, nil)
					go func() { _, _ = io.Copy(stdin, ch); stdin.Close() }()
					status := 0
					if err := cmd.Run(); err != nil {
						status = 1
						var ee *exec.ExitError
						if errors.As(err, &ee) {
							status = ee.ExitCode()
						}
					}
					_, _ = ch.SendRequest("exit-status", false, binary.BigEndian.AppendUint32(nil, uint32(status)))
					return
				case "signal":
					if cmd != nil && cmd.Process != nil {
						_ = cmd.Process.Kill()
					}
				default:
					_ = req.Reply(false, nil)
				}
			}
		}()
	}
}

func writeClientKey(t *testing.T) (string, ssh.PublicKey) {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	sshPub, _ := ssh.NewPublicKey(pub)
	return path, sshPub
}

func dialTest(t *testing.T, srv *testServer, keyPath string) (*Client, error) {
	host, portStr, _ := net.SplitHostPort(srv.addr)
	port, _ := strconv.Atoi(portStr)
	return Dial(ClientOptions{Host: host, Port: port, User: "test", PrivateKeyPath: keyPath, Timeout: 5 * time.Second})
}

func TestRunnerAgainstRealSSHServer(t *testing.T) {
	t.Setenv("TERANGAHOST_HOME", t.TempDir())
	t.Setenv("SSH_AUTH_SOCK", "")
	keyPath, pub := writeClientKey(t)
	srv := startTestServer(t, pub)

	client, err := dialTest(t, srv, keyPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if got := client.AuthorizedKey(); got != strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub))) {
		t.Errorf("AuthorizedKey = %q", got)
	}
	r := NewRunner(client, RunnerOptions{})
	defer r.Close()
	ctx := context.Background()

	out, err := r.RunSilent(ctx, `printf '%s' "it's ok" | tr a-z A-Z`)
	if err != nil || out != "IT'S OK" {
		t.Errorf("RunSilent = %q, %v", out, err)
	}

	if _, err := r.RunSilent(ctx, "echo détail >&2; exit 3"); err == nil || !errors.Is(err, domain.ErrCommandExecution) || !strings.Contains(err.Error(), "détail") {
		t.Errorf("l'erreur doit contenir stderr: %v", err)
	}

	if out, err := r.RunWithInput(ctx, "cat | wc -c | tr -d ' '", []byte("secret\n")); err != nil || out != "7" {
		t.Errorf("RunWithInput = %q, %v", out, err)
	}

	dest := filepath.Join(t.TempDir(), "sub dir", "file.conf")
	if err := r.Upload(ctx, []byte("contenu\n"), dest, 0o640); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	data, _ := os.ReadFile(dest)
	st, _ := os.Stat(dest)
	if string(data) != "contenu\n" || st.Mode().Perm() != 0o640 {
		t.Errorf("Upload: contenu %q, mode %v", data, st.Mode().Perm())
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(dest), ".terangahost.*"))
	if len(leftovers) != 0 {
		t.Errorf("fichiers temporaires restants: %v", leftovers)
	}
	if ok, _ := r.FileExists(ctx, dest); !ok {
		t.Error("FileExists devrait être vrai")
	}
	if ok, _ := r.FileExists(ctx, dest+".absent"); ok {
		t.Error("FileExists devrait être faux")
	}

	cctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := r.RunSilent(cctx, "sleep 10"); !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 3*time.Second {
		t.Errorf("l'annulation du contexte doit interrompre la commande: %v après %s", err, time.Since(start))
	}
}

func TestHostKeyChangeIsRejected(t *testing.T) {
	t.Setenv("TERANGAHOST_HOME", t.TempDir())
	t.Setenv("SSH_AUTH_SOCK", "")
	keyPath, pub := writeClientKey(t)
	srv := startTestServer(t, pub)
	c, err := dialTest(t, srv, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()

	// Simule une réinstallation du serveur : l'empreinte mémorisée ne correspond plus.
	path, _ := KnownHostsPath()
	data, _ := os.ReadFile(path)
	fields := strings.Fields(string(data))
	tampered := fields[0] + " " + fields[1] + " " + strings.Repeat("0", 64) + "\n"
	_ = os.WriteFile(path, []byte(tampered), 0o600)

	if _, err := dialTest(t, srv, keyPath); !errors.Is(err, domain.ErrHostKeyMismatch) {
		t.Fatalf("attendu ErrHostKeyMismatch, obtenu %v", err)
	}
	if n, err := ForgetHost(srv.addr); err != nil || n != 1 {
		t.Fatalf("ForgetHost = %d, %v", n, err)
	}
	c, err = dialTest(t, srv, keyPath)
	if err != nil {
		t.Fatalf("après forget-host la connexion doit réussir: %v", err)
	}
	c.Close()
}

func TestWrongKeyIsRejected(t *testing.T) {
	t.Setenv("TERANGAHOST_HOME", t.TempDir())
	t.Setenv("SSH_AUTH_SOCK", "")
	_, pub := writeClientKey(t)
	otherKey, _ := writeClientKey(t)
	srv := startTestServer(t, pub)
	if _, err := dialTest(t, srv, otherKey); !errors.Is(err, domain.ErrSSHAuthentication) {
		t.Fatalf("attendu ErrSSHAuthentication, obtenu %v", err)
	}
}
