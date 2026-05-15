// Package rust writes ~/.cargo/config.toml so Cargo fetches crates from
// a Chinese registry mirror, using the sparse-index protocol when the
// installed Cargo supports it.
package rust

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/loredunk/china-mirror/internal/adapter"
	"github.com/loredunk/china-mirror/internal/config"
	"github.com/loredunk/china-mirror/internal/mirrors"
)

func init() { adapter.Register(&Adapter{}) }

type Adapter struct{}

func (a *Adapter) Name() string         { return "rust" }
func (a *Adapter) Categories() []string { return []string{"cargo"} }
func (a *Adapter) Description() string {
	return "Configure Cargo to fetch crates from a Chinese mirror"
}
func (a *Adapter) BackupTargets() []string { return config.ToolConfigs["cargo"] }
func (a *Adapter) Commands() []adapter.Command {
	return []adapter.Command{
		{Name: "setup", Description: "Write ~/.cargo/config.toml pointing at the chosen crates mirror"},
	}
}

func (a *Adapter) Run(cmd string, opts adapter.Options) error {
	switch cmd {
	case "setup":
		return a.setup(opts)
	}
	return fmt.Errorf("rust: unknown command %q", cmd)
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

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dst := filepath.Join(home, ".cargo", "config.toml")
	legacy := filepath.Join(home, ".cargo", "config")
	if fileExists(legacy) && !fileExists(dst) {
		dst = legacy
	}

	sparse := supportsSparseIndex()
	short := strings.TrimSuffix(strings.TrimPrefix(mirror.ID, "cargo-"), "/")
	if short == "" {
		short = "china-mirror"
	}

	var content string
	if sparse {
		content = fmt.Sprintf(`# Cargo mirror configuration - written by china-mirror-cli
# Mirror: %s (%s)

[registries.crates-io]
index = "sparse+%s/"

[net]
git-fetch-with-cli = true
`, mirror.ID, mirror.Name, strings.TrimRight(mirror.URL, "/"))
	} else {
		content = fmt.Sprintf(`# Cargo mirror configuration - written by china-mirror-cli
# Mirror: %s (%s)
# NOTE: Cargo <1.68 — falling back to git-based index

[source.crates-io]
replace-with = '%s'

[source.%s]
registry = "%s"

[net]
git-fetch-with-cli = true
`, mirror.ID, mirror.Name, short, short, mirror.URL)
	}

	fmt.Printf("Using mirror %s (%s) — %s\n", mirror.ID, mirror.Name, mirror.URL)
	if sparse {
		fmt.Println("  cargo supports sparse index — using sparse+ protocol")
	} else {
		fmt.Println("  cargo <1.68 detected (or not installed) — using legacy git index")
	}
	return writeConfig(dst, []byte(content), opts)
}

// supportsSparseIndex returns true when `cargo --version` reports 1.68+.
// Missing cargo also returns true (we assume modern install).
func supportsSparseIndex() bool {
	out, err := exec.Command("cargo", "--version").Output()
	if err != nil {
		return true
	}
	re := regexp.MustCompile(`(\d+)\.(\d+)`)
	m := re.FindStringSubmatch(string(out))
	if len(m) < 3 {
		return true
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return major > 1 || (major == 1 && minor >= 68)
}

func resolveMirror(store *mirrors.Store, id string) (mirrors.Mirror, error) {
	if id != "" {
		m, ok := store.Get(id)
		if !ok {
			return mirrors.Mirror{}, fmt.Errorf("unknown mirror id %q (try `cmc list mirrors --category cargo`)", id)
		}
		if m.Category != "cargo" {
			return mirrors.Mirror{}, fmt.Errorf("mirror %q is in category %q, not cargo", id, m.Category)
		}
		return m, nil
	}
	m, ok := store.Default("cargo")
	if !ok {
		return mirrors.Mirror{}, fmt.Errorf("no active cargo mirror in mirrors.yml")
	}
	return m, nil
}

func writeConfig(dst string, content []byte, opts adapter.Options) error {
	if opts.DryRun {
		fmt.Printf("[dry-run] would write %s:\n", dst)
		fmt.Println(indent(string(content), "    "))
		return nil
	}
	if fileExists(dst) {
		dir, err := config.BackupFile(dst, "cargo")
		if err != nil {
			return fmt.Errorf("backup %s: %w", dst, err)
		}
		if dir != "" {
			fmt.Printf("  backed up %s → %s\n", dst, dir)
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, content, 0o644); err != nil {
		return err
	}
	fmt.Printf("✓ cargo config written: %s\n", dst)
	return nil
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
