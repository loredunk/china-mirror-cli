// Package golang sets GOPROXY in the user's shell profile so `go mod
// download` fetches from a Chinese module proxy. Registered under the
// adapter name "go".
package golang

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/loredunk/china-mirror/internal/adapter"
	"github.com/loredunk/china-mirror/internal/config"
	"github.com/loredunk/china-mirror/internal/mirrors"
)

func init() { adapter.Register(&Adapter{}) }

const (
	blockMarker = "# china-mirror-cli: go"
	blockEnd    = "# end china-mirror-cli: go"
)

type Adapter struct{}

func (a *Adapter) Name() string         { return "go" }
func (a *Adapter) Categories() []string { return []string{"go"} }
func (a *Adapter) Description() string {
	return "Set GOPROXY to a Chinese Go module proxy in your shell profile"
}
func (a *Adapter) BackupTargets() []string { return config.ToolConfigs["go"] }
func (a *Adapter) Commands() []adapter.Command {
	return []adapter.Command{
		{Name: "setup", Description: "Append GOPROXY exports to the active shell profile"},
	}
}

func (a *Adapter) Run(cmd string, opts adapter.Options) error {
	switch cmd {
	case "setup":
		return a.setup(opts)
	}
	return fmt.Errorf("go: unknown command %q", cmd)
}

func (a *Adapter) setup(opts adapter.Options) error {
	store, err := mirrors.Load()
	if err != nil {
		return fmt.Errorf("load mirrors: %w", err)
	}
	mirror, err := resolveMirror(store, opts.Mirror)
	if err != nil {
		return err
	}
	proxyURL := strings.TrimRight(mirror.URL, "/")

	block := fmt.Sprintf(`%s
export GOPROXY="%s,direct"
export GO111MODULE=on
%s
`, blockMarker, proxyURL, blockEnd)

	fmt.Printf("Using proxy %s (%s) — %s\n", mirror.ID, mirror.Name, mirror.URL)

	profile, err := pickShellProfile()
	if err != nil {
		return err
	}
	if err := writeProfileBlock(profile, block, opts); err != nil {
		return err
	}

	if !opts.DryRun && commandExists("go") {
		fmt.Println("  current `go env GOPROXY`:")
		if out, err := exec.Command("go", "env", "GOPROXY").Output(); err == nil {
			fmt.Println("    " + strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func resolveMirror(store *mirrors.Store, id string) (mirrors.Mirror, error) {
	if id != "" {
		m, ok := store.Get(id)
		if !ok {
			return mirrors.Mirror{}, fmt.Errorf("unknown mirror id %q (try `cmc list mirrors --category go`)", id)
		}
		if m.Category != "go" {
			return mirrors.Mirror{}, fmt.Errorf("mirror %q is in category %q, not go", id, m.Category)
		}
		return m, nil
	}
	m, ok := store.Default("go")
	if !ok {
		return mirrors.Mirror{}, fmt.Errorf("no active go mirror in mirrors.yml")
	}
	return m, nil
}

func pickShellProfile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	shell := os.Getenv("SHELL")
	switch {
	case strings.Contains(shell, "zsh"):
		return filepath.Join(home, ".zshrc"), nil
	case strings.Contains(shell, "bash"):
		p := filepath.Join(home, ".bashrc")
		if !fileExists(p) {
			p = filepath.Join(home, ".bash_profile")
		}
		return p, nil
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); err == nil {
		return filepath.Join(home, ".zshrc"), nil
	}
	return filepath.Join(home, ".bashrc"), nil
}

func writeProfileBlock(path, block string, opts adapter.Options) error {
	if opts.DryRun {
		fmt.Printf("[dry-run] would append to %s:\n", path)
		fmt.Println(indent(block, "    "))
		return nil
	}
	if fileExists(path) {
		dir, err := config.BackupFile(path, "go")
		if err != nil {
			return fmt.Errorf("backup %s: %w", path, err)
		}
		if dir != "" {
			fmt.Printf("  backed up %s → %s\n", path, dir)
		}
	}
	data, _ := os.ReadFile(path)
	cleaned := stripOldBlock(string(data))
	cleaned = strings.TrimRight(cleaned, "\n") + "\n\n" + block
	if err := os.WriteFile(path, []byte(cleaned), 0o644); err != nil {
		return err
	}
	fmt.Printf("✓ GOPROXY appended to %s\n", path)
	fmt.Printf("  hint: run `source %s` or restart your shell\n", path)
	return nil
}

func stripOldBlock(content string) string {
	start := strings.Index(content, blockMarker)
	if start < 0 {
		return content
	}
	rest := content[start:]
	endIdx := strings.Index(rest, blockEnd)
	if endIdx < 0 {
		return content[:start]
	}
	endIdx += len(blockEnd)
	return content[:start] + content[start+endIdx:]
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

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
