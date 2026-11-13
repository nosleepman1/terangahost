package validate

import "testing"

func TestDomain(t *testing.T) {
	ok := map[string]string{
		"api.monprojet.sn":   "api.monprojet.sn",
		"API.Example.COM.":   "api.example.com",
		"xn--bcher-kva.ch":   "xn--bcher-kva.ch",
		"a-b.c-d.example.io": "a-b.c-d.example.io",
	}
	for in, want := range ok {
		got, err := Domain(in)
		if err != nil || got != want {
			t.Errorf("Domain(%q) = %q, %v ; attendu %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "localhost", "a..b", "-a.com", "a-.com", "a.com; rm -rf /", "a b.com", "1.2.3.4", "../etc.com", "a_b.com"} {
		if _, err := Domain(bad); err == nil {
			t.Errorf("Domain(%q) aurait dû échouer", bad)
		}
	}
}

func TestBranch(t *testing.T) {
	for _, b := range []string{"main", "release/1.2", "feat-x_y", "v1.0.0"} {
		if err := Branch(b); err != nil {
			t.Errorf("Branch(%q): %v", b, err)
		}
	}
	for _, b := range []string{"", "-x", "a..b", "a b", "x;id", "$(id)", "a.lock", "a/"} {
		if Branch(b) == nil {
			t.Errorf("Branch(%q) aurait dû échouer", b)
		}
	}
}

func TestRepository(t *testing.T) {
	for _, r := range []string{"https://github.com/org/app.git", "git@github.com:org/app.git", "ssh://git@gitlab.com:22/org/app.git"} {
		if err := Repository(r); err != nil {
			t.Errorf("Repository(%q): %v", r, err)
		}
	}
	for _, r := range []string{"", "http://x.com/a.git", "--upload-pack=touch /tmp/x", "https://x.com/a b", "https://x.com/$(id)", "file:///etc", "/local/path"} {
		if Repository(r) == nil {
			t.Errorf("Repository(%q) aurait dû échouer", r)
		}
	}
}

func TestDatabase(t *testing.T) {
	cases := map[string]string{"mysql": "mariadb", "MariaDB": "mariadb", "postgresql": "postgres", "none": "none"}
	for in, want := range cases {
		if got, err := Database(in); err != nil || got != want {
			t.Errorf("Database(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := Database("mongodb"); err == nil {
		t.Error("mongodb devrait être refusé")
	}
}

func TestServerNameAndHost(t *testing.T) {
	if ServerName("dakar-prod_01") != nil || ServerName("Bad Name") == nil || ServerName("") == nil {
		t.Error("validation ServerName incorrecte")
	}
	if Host("192.168.1.50") != nil || Host("2001:db8::1") != nil || Host("vps.example.com") != nil || Host("x;y") == nil {
		t.Error("validation Host incorrecte")
	}
	if PHPVersion("8.3") != nil || PHPVersion("7.4") == nil {
		t.Error("validation PHPVersion incorrecte")
	}
}
