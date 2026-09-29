package shell

import (
	"os/exec"
	"testing"
)

func TestQuote(t *testing.T) {
	cases := map[string]string{
		"":                   "''",
		"simple":             "simple",
		"/var/www/a.b":       "/var/www/a.b",
		"a b":                "'a b'",
		"it's":               `'it'\''s'`,
		"$(rm -rf /)":        "'$(rm -rf /)'",
		"x; reboot":          "'x; reboot'",
		"git@github.com:a/b": "git@github.com:a/b",
	}
	for in, want := range cases {
		if got := Quote(in); got != want {
			t.Errorf("Quote(%q) = %q, want %q", in, got, want)
		}
	}
}

// Vérifie avec un vrai bash que la valeur échappée est reçue à l'identique.
func TestQuoteRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash indisponible")
	}
	for _, in := range []string{"a b", "it's", "$(id)", "`id`", "a\nb", `\"'`, "--help", "*"} {
		out, err := exec.Command("bash", "-c", "printf %s "+Quote(in)).Output()
		if err != nil {
			t.Fatalf("bash: %v", err)
		}
		if string(out) != in {
			t.Errorf("round trip de %q: obtenu %q", in, out)
		}
	}
}
