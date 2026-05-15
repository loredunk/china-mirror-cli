// Package homebrew configures Homebrew to fetch from a Chinese mirror by
// exporting HOMEBREW_* env vars in the user's shell profile and rewriting
// the git remotes of an existing brew install.
package homebrew

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

const blockMarker = "# china-mirror-cli: homebrew"

type Adapter struct{}

func (a *Adapter) Name() string         { return "homebrew" }
func (a *Adapter) Categories() []string { return []string{"homebrew"} }
func (a *Adapter) Description() string {
	return "Configure Homebrew (brew) to use a Chinese mirror"
}
func (a *Adapter) BackupTargets() []string { return config.ToolConfigs["homebrew"] }
func (a *Adapter) Commands() []adapter.Command {
	return []adapter.Command{
		{Name: "setup", Description: "Export HOMEBREW_* env vars and rewrite git remotes"},
	}
}

func (a *Adapter) Run(cmd string, opts adapter.Options) error {
	switch cmd {
	case "setup":
		return a.setup(opts)
	}
	return fmt.Errorf("homebrew: unknown command %q", cmd)
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

	base := strings.TrimRight(mirror.URL, "/")
	brewRemote, coreRemote, bottleDomain := brewURLs(mirror.ID, base)

	exports := fmt.Sprintf(`%s
export HOMEBREW_BREW_GIT_REMOTE="%s"
export HOMEBREW_CORE_GIT_REMOTE="%s"
export HOMEBREW_BOTTLE_DOMAIN="%s"
# end china-mirror-cli: homebrew
`, blockMarker, brewRemote, coreRemote, bottleDomain)

	fmt.Printf("Using mirror %s (%s) — %s\n", mirror.ID, mirror.Name, mirror.URL)

	profile, err := pickShellProfile()
	if err != nil {
		return err
	}
	if err := writeProfileBlock(profile, exports, opts); err != nil {
		return err
	}

	if !commandExists("brew") {
		fmt.Println("  hint: brew is not installed yet. Install it after sourcing the profile.")
		return nil
	}
	if opts.DryRun {
		fmt.Println("[dry-run] would rewrite existing brew git remotes")
		return nil
	}
	return rewriteRemotes(brewRemote, coreRemote)
}

func brewURLs(mirrorID, base string) (brew, core, bottle string) {
	switch mirrorID {
	case "brew-ustc":
		// USTC ships the brew repo directly as .git endpoints rooted at brew.git.
		return base, strings.Replace(base, "brew.git", "homebrew-core.git", 1),
			"https://mirrors.ustc.edu.cn/homebrew-bottles"
	default:
		// TUNA-style: base/<repo>.git layout. `base` has trailing slash trimmed,
		// so explicitly join with "/" to avoid "homebrewbrew.git".
		root := strings.TrimSuffix(base, "/git/homebrew")
		return base + "/brew.git", base + "/homebrew-core.git", root + "/homebrew-bottles"
	}
}

func rewriteRemotes(brewRemote, coreRemote string) error {
	prefix, err := runCmdOut("brew", "--prefix")
	if err != nil {
		return fmt.Errorf("brew --prefix: %w", err)
	}
	prefix = strings.TrimSpace(prefix)

	for _, target := range []struct {
		dir, remote string
	}{
		{filepath.Join(prefix), brewRemote},
		{filepath.Join(prefix, "Library", "Taps", "homebrew", "homebrew-core"), coreRemote},
	} {
		if !fileExists(filepath.Join(target.dir, ".git")) {
			continue
		}
		if err := runCmd("git", "-C", target.dir, "remote", "set-url", "origin", target.remote); err != nil {
			fmt.Printf("  warn: could not rewrite remote in %s: %v\n", target.dir, err)
			continue
		}
		fmt.Printf("✓ rewrote origin in %s → %s\n", target.dir, target.remote)
	}
	return nil
}

func resolveMirror(store *mirrors.Store, id string) (mirrors.Mirror, error) {
	if id != "" {
		m, ok := store.Get(id)
		if !ok {
			return mirrors.Mirror{}, fmt.Errorf("unknown mirror id %q (try `cmc list mirrors --category homebrew`)", id)
		}
		if m.Category != "homebrew" {
			return mirrors.Mirror{}, fmt.Errorf("mirror %q is in category %q, not homebrew", id, m.Category)
		}
		return m, nil
	}
	m, ok := store.Default("homebrew")
	if !ok {
		return mirrors.Mirror{}, fmt.Errorf("no active homebrew mirror in mirrors.yml")
	}
	return m, nil
}

// --- shared helpers (also used by go/flutter adapters; intentionally
// duplicated for now to keep adapters self-contained, matching the
// existing python/node pattern).

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
		p := filepath.Join(home, ".bash_profile")
		if !fileExists(p) {
			p = filepath.Join(home, ".bashrc")
		}
		return p, nil
	}
	// Fallback: prefer .zshrc on macOS, .bashrc elsewhere.
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
		dir, err := config.BackupFile(path, "homebrew")
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
	fmt.Printf("✓ homebrew env appended to %s\n", path)
	fmt.Printf("  hint: run `source %s` or restart your shell\n", path)
	return nil
}

func stripOldBlock(content string) string {
	start := strings.Index(content, blockMarker)
	if start < 0 {
		return content
	}
	endMarker := "# end china-mirror-cli: homebrew"
	rest := content[start:]
	endIdx := strings.Index(rest, endMarker)
	if endIdx < 0 {
		return content[:start]
	}
	endIdx += len(endMarker)
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

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCmdOut(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return string(out), err
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
