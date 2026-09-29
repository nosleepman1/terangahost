package ssh

import (
	"os/exec"
	"strings"
	"testing"
)

func TestWrapProducesValidBash(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash indisponible")
	}
	root := &Runner{}
	sudo := &Runner{opts: RunnerOptions{Sudo: true}}
	cmd := `echo "it's $HOME" | grep -q x && printf '%s\n' 'a b'`
	if w := sudo.wrap(cmd); !strings.HasPrefix(w, "sudo -n bash -c ") {
		t.Errorf("wrap sudo = %s", w)
	}
	out, err := exec.Command("bash", "-c", root.wrap(`printf '%s|' "it's" "$DEBIAN_FRONTEND"; false | true`)).CombinedOutput()
	if err == nil || string(out) != "it's|noninteractive|" {
		t.Errorf("wrap doit préserver les guillemets, exporter DEBIAN_FRONTEND et activer pipefail: out=%q err=%v", out, err)
	}
	if out, err := exec.Command("bash", "-n", "-c", root.wrap(cmd)).CombinedOutput(); err != nil {
		t.Errorf("syntaxe invalide: %v %s", err, out)
	}
}
