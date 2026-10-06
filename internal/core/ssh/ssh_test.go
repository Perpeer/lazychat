package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArgv(t *testing.T) {
	cases := []struct {
		name string
		c    Conn
		want string
	}{
		{"agent, default port", Conn{Host: "garden-shed", User: "gardener", Auth: Agent}, "ssh gardener@garden-shed"},
		{"key file, another port", Conn{Host: "garden-shed", User: "gardener", Port: 2222, Auth: KeyFile, Key: "/keys/id_ed25519"}, "ssh -p 2222 -i /keys/id_ed25519 -o IdentitiesOnly=yes gardener@garden-shed"},
		{"password", Conn{Host: "garden-shed", Auth: Password}, "ssh -o PubkeyAuthentication=no -o PreferredAuthentications=keyboard-interactive,password garden-shed"},
		{"a config host", Conn{Alias: "shed-pi", Host: "ignored", Port: 2222}, "ssh shed-pi"},
		{"a config host with a key", Conn{Alias: "shed-pi", Auth: KeyFile, Key: "/keys/id_rsa"}, "ssh -i /keys/id_rsa shed-pi"},
	}
	for _, tc := range cases {
		if got := strings.Join(Argv(tc.c), " "); got != tc.want {
			t.Errorf("%s:\n got  %s\n want %s", tc.name, got, tc.want)
		}
	}
	if got := Target(Conn{Host: "garden-shed", User: "gardener", Port: 2222}); got != "gardener@garden-shed:2222" {
		t.Errorf("target %q", got)
	}
}

// Keys are the files with a .pub beside them, any type, as ~-paths; a
// lone .pub, known_hosts and folders are not keys.
func TestKeys(t *testing.T) {
	dir := filepath.Join(os.Getenv("HOME"), ".ssh")
	if err := os.MkdirAll(filepath.Join(dir, "sockets"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"id_ed25519", "id_ed25519.pub", "shed_deploy", "shed_deploy.pub", "id_rsa", "id_rsa.pub", "known_hosts", "config", "orphan.pub"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := strings.Join(Keys(Dir()), " ")
	if want := "~/.ssh/id_ed25519 ~/.ssh/id_rsa ~/.ssh/shed_deploy"; got != want {
		t.Errorf("keys %q, want %q", got, want)
	}
	if Keys(filepath.Join(dir, "missing")) != nil {
		t.Error("a missing folder has keys")
	}
}

func TestConfigHosts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	config := "# the garden\nHost shed-pi\n  HostName garden-shed\n  User gardener\n\nHost garden-box fence-*\nhost shed-pi\nHost !gate\nMatch all\nHost *\n"
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ConfigHosts(path), " "); got != "shed-pi garden-box" {
		t.Errorf("hosts %q", got)
	}
	if ConfigHosts(filepath.Join(t.TempDir(), "none")) != nil {
		t.Error("a missing config has hosts")
	}
}
