// Package docker configures Docker to pull images through a Chinese
// Docker Hub mirror by rewriting /etc/docker/daemon.json. Requires root.
//
// Note: Docker CE *install* mirrors (the APT repo for installing the
// docker-ce package itself) are a separate concern and not handled here —
// the bash scripts/docker/setup.sh covers that.
package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/loredunk/china-mirror/internal/adapter"
	"github.com/loredunk/china-mirror/internal/config"
	"github.com/loredunk/china-mirror/internal/mirrors"
)

func init() { adapter.Register(&Adapter{}) }

const daemonJSON = "/etc/docker/daemon.json"

type Adapter struct{}

func (a *Adapter) Name() string         { return "docker" }
func (a *Adapter) Categories() []string { return []string{"docker-hub"} }
func (a *Adapter) Description() string {
	return "Point Docker daemon at a Chinese Docker Hub mirror (registry-mirrors in daemon.json)"
}
func (a *Adapter) BackupTargets() []string { return config.ToolConfigs["docker"] }
func (a *Adapter) Commands() []adapter.Command {
	return []adapter.Command{
		{Name: "setup", Description: "Rewrite /etc/docker/daemon.json with registry-mirrors (requires root)"},
	}
}

func (a *Adapter) Run(cmd string, opts adapter.Options) error {
	switch cmd {
	case "setup":
		return a.setup(opts)
	}
	return fmt.Errorf("docker: unknown command %q", cmd)
}

func (a *Adapter) setup(opts adapter.Options) error {
	if runtime.GOOS != "linux" {
		fmt.Println("note: editing /etc/docker/daemon.json is only meaningful on Linux Docker hosts.")
		fmt.Println("      on macOS/Windows use Docker Desktop's settings UI.")
	}

	store, err := mirrors.Load()
	if err != nil {
		return fmt.Errorf("load mirrors: %w", err)
	}

	// Pick top-N active docker-hub mirrors so the daemon can fall back.
	mirrorList, err := resolveMirrors(store, opts.Mirror)
	if err != nil {
		return err
	}
	urls := make([]string, 0, len(mirrorList))
	for _, m := range mirrorList {
		urls = append(urls, strings.TrimRight(m.URL, "/"))
	}

	fmt.Println("Using docker-hub mirrors:")
	for _, m := range mirrorList {
		fmt.Printf("  - %s (%s) — %s\n", m.ID, m.Name, m.URL)
	}

	content, err := buildDaemonJSON(urls)
	if err != nil {
		return err
	}

	if opts.DryRun {
		fmt.Println("[dry-run] would write " + daemonJSON + ":")
		fmt.Println(indent(content, "    "))
		return nil
	}

	if runtime.GOOS == "linux" && !isRoot() {
		return fmt.Errorf("writing %s requires root (run with sudo)", daemonJSON)
	}

	if fileExists(daemonJSON) {
		dir, err := config.BackupFile(daemonJSON, "docker")
		if err != nil {
			return fmt.Errorf("backup %s: %w", daemonJSON, err)
		}
		if dir != "" {
			fmt.Printf("  backed up %s → %s\n", daemonJSON, dir)
		}
	}
	if err := os.MkdirAll("/etc/docker", 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(daemonJSON, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Printf("✓ docker config written: %s\n", daemonJSON)
	fmt.Println("  hint: restart the docker daemon to apply (sudo systemctl restart docker)")
	return nil
}

// buildDaemonJSON merges new registry-mirrors into an existing daemon.json
// (preserving the user's other keys) or creates one from scratch.
func buildDaemonJSON(urls []string) (string, error) {
	cfg := map[string]any{}
	if data, err := os.ReadFile(daemonJSON); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return "", fmt.Errorf("parse existing %s: %w", daemonJSON, err)
		}
	}
	cfg["registry-mirrors"] = urls
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}

// resolveMirrors returns either the single requested mirror or every
// active docker-hub mirror (priority-sorted) so the daemon can fall back.
func resolveMirrors(store *mirrors.Store, id string) ([]mirrors.Mirror, error) {
	if id != "" {
		m, ok := store.Get(id)
		if !ok {
			return nil, fmt.Errorf("unknown mirror id %q (try `cmc list mirrors --category docker-hub`)", id)
		}
		if m.Category != "docker-hub" {
			return nil, fmt.Errorf("mirror %q is in category %q, not docker-hub", id, m.Category)
		}
		return []mirrors.Mirror{m}, nil
	}
	all := store.ByCategory("docker-hub")
	out := make([]mirrors.Mirror, 0, len(all))
	for _, m := range all {
		if m.Status == mirrors.StatusActive {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no active docker-hub mirror in mirrors.yml")
	}
	return out, nil
}

func isRoot() bool { return os.Geteuid() == 0 }

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

