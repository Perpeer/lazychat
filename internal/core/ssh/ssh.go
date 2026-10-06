// Package ssh is what lazychat knows of a saved SSH connection: the command
// line that opens it, the keys in the user's .ssh folder, the hosts their
// ssh config names. The connection itself is OpenSSH's own ssh in a pty, so
// known_hosts, the agent, ProxyJump and the user's config work as they do in
// any terminal; nothing here talks to a network or keeps a password. Paths
// come from the user's home alone, so the same code serves Linux.
package ssh

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"lazychat/internal/core/files"
)

// Program is the ssh a connection runs; the tests point it at a stand-in.
var Program = "ssh"

// How a connection signs in. A password is never kept: ssh asks for it in
// the pane each time.
const (
	KeyFile  = "key"
	Agent    = "agent"
	Password = "password"
)

// Conn is a connection as it is saved: Alias, when set, is a Host of the
// user's ssh config, and then the config says the rest.
type Conn struct {
	Host, User, Alias string
	Port              int
	Auth, Key         string
}

// Argv is ssh's command line for c.
func Argv(c Conn) []string {
	argv := []string{Program}
	if c.Alias != "" {
		if c.Auth == KeyFile && c.Key != "" {
			argv = append(argv, "-i", files.ExpandHome(c.Key))
		}
		return append(argv, c.Alias)
	}
	if c.Port != 0 && c.Port != 22 {
		argv = append(argv, "-p", strconv.Itoa(c.Port))
	}
	switch c.Auth {
	case KeyFile:
		if c.Key != "" {
			// Only that key: an agent holding many is refused after too
			// many tries before the right one comes.
			argv = append(argv, "-i", files.ExpandHome(c.Key), "-o", "IdentitiesOnly=yes")
		}
	case Password:
		argv = append(argv, "-o", "PubkeyAuthentication=no", "-o", "PreferredAuthentications=keyboard-interactive,password")
	}
	target := c.Host
	if c.User != "" {
		target = c.User + "@" + c.Host
	}
	return append(argv, target)
}

// Target is how a connection reads in a title: user@host:port, or its alias.
func Target(c Conn) string {
	if c.Alias != "" {
		return c.Alias
	}
	t := c.Host
	if c.User != "" {
		t = c.User + "@" + t
	}
	if c.Port != 0 && c.Port != 22 {
		t += ":" + strconv.Itoa(c.Port)
	}
	return t
}

// Dir is the user's .ssh folder.
func Dir() string { return filepath.Join(files.Home(), ".ssh") }

// Keys are the private keys in dir — a file with its .pub beside it, of any
// type — as ~-paths, sorted; none when the folder is missing.
func Keys(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasSuffix(name, ".pub") {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, name+".pub")); err == nil {
			out = append(out, shortHome(filepath.Join(dir, name)))
		}
	}
	sort.Strings(out)
	return out
}

// ConfigHosts are the Host names of an ssh config, in its order, patterns
// (with * or ?) and negations left out; none when it cannot be read.
func ConfigHosts(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || !strings.EqualFold(fields[0], "host") {
			continue
		}
		for _, h := range fields[1:] {
			if strings.ContainsAny(h, "*?!") || seen[h] {
				continue
			}
			seen[h] = true
			out = append(out, h)
		}
	}
	return out
}

func shortHome(p string) string {
	if home := files.Home(); home != "" && strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}
