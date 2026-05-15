// Package conda rewrites ~/.condarc to use a Chinese Anaconda mirror.
package conda

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

type Adapter struct{}

func (a *Adapter) Name() string         { return "conda" }
func (a *Adapter) Categories() []string { return []string{"conda"} }
func (a *Adapter) Description() string {
	return "Rewrite ~/.condarc to use a Chinese Anaconda mirror"
}
func (a *Adapter) BackupTargets() []string { return config.ToolConfigs["conda"] }
func (a *Adapter) Commands() []adapter.Command {
	return []adapter.Command{
		{Name: "setup", Description: "Write a .condarc that points at the chosen mirror"},
	}
}

func (a *Adapter) Run(cmd string, opts adapter.Options) error {
	switch cmd {
	case "setup":
		return a.setup(opts)
	}
	return fmt.Errorf("conda: unknown command %q", cmd)
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

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dst := filepath.Join(home, ".condarc")

	content := fmt.Sprintf(`# Conda mirror configuration - written by china-mirror-cli
# Mirror: %s (%s)

channels:
  - defaults
show_channel_urls: true
default_channels:
  - %s/pkgs/main
  - %s/pkgs/r
  - %s/pkgs/msys2
custom_channels:
  conda-forge: %s/cloud
  bioconda: %s/cloud
  menpo: %s/cloud
  pytorch: %s/cloud
  pytorch-lts: %s/cloud
  simpleitk: %s/cloud
`, mirror.ID, mirror.Name, base, base, base, base, base, base, base, base, base)

	fmt.Printf("Using mirror %s (%s) — %s\n", mirror.ID, mirror.Name, mirror.URL)
	if err := writeConfig(dst, []byte(content), opts); err != nil {
		return err
	}

	if !opts.DryRun && (commandExists("conda") || commandExists("mamba")) {
		bin := "conda"
		if !commandExists("conda") {
			bin = "mamba"
		}
		fmt.Println("  cleaning conda index cache…")
		_ = runCmd(bin, "clean", "-i", "-y")
	}
	return nil
}

func resolveMirror(store *mirrors.Store, id string) (mirrors.Mirror, error) {
	if id != "" {
		m, ok := store.Get(id)
		if !ok {
			return mirrors.Mirror{}, fmt.Errorf("unknown mirror id %q (try `cmc list mirrors --category conda`)", id)
		}
		if m.Category != "conda" {
			return mirrors.Mirror{}, fmt.Errorf("mirror %q is in category %q, not conda", id, m.Category)
		}
		return m, nil
	}
	m, ok := store.Default("conda")
	if !ok {
		return mirrors.Mirror{}, fmt.Errorf("no active conda mirror in mirrors.yml")
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
		dir, err := config.BackupFile(dst, "conda")
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
	fmt.Printf("✓ conda config written: %s\n", dst)
	return nil
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

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
