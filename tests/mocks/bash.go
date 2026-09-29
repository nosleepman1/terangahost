package mocks

import (
	"os/exec"
	"strings"
	"testing"
)

// AssertBashSyntax vérifie avec "bash -n" que chaque commande enregistrée est syntaxiquement valide.
func AssertBashSyntax(t *testing.T, m *MockRunner) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash indisponible")
	}
	m.mu.Lock()
	cmds := append([]string(nil), m.Commands...)
	m.mu.Unlock()
	for _, c := range cmds {
		if strings.HasPrefix(c, "upload ") {
			continue
		}
		if out, err := exec.Command("bash", "-n", "-c", c).CombinedOutput(); err != nil {
			t.Errorf("syntaxe bash invalide: %v\n%s\n--- commande ---\n%s", err, out, c)
		}
	}
}
