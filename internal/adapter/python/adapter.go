// Package python configures pip, uv and poetry to use a Chinese PyPI
// mirror. It registers itself with the global adapter registry on
// import.
package python

import (
	"fmt"
	"net/url"
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

func (a *Adapter) Name() string          { return "python" }
func (a *Adapter) Categories() []string  { return []string{"pip"} }
func (a *Adapter) Description() string   { return "Configure pip / uv / poetry to use a Chinese PyPI mirror" }
func (a *Adapter) BackupTargets() []string {
	return append(append(config.ToolConfigs["pip"], config.ToolConfigs["uv"]...), config.ToolConfigs["poetry"]...)
}
func (a *Adapter) Commands() []adapter.Command {
	return []adapter.Command{
		{Name: "setup", Description: "Point installed Python package managers at a China mirror"},
	}
}

func (a *Adapter) Run(cmd string, opts adapter.Options) error {
	switch cmd {
	case "setup":
		return a.setup(opts)
	}
	return fmt.Errorf("python: unknown command %q", cmd)
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
	host, err := hostOf(mirror.URL)
	if err != nil {
		return fmt.Errorf("parse mirror url: %w", err)
	}

	tool := opts.Extra["tool"] // "", "pip", "uv", "poetry", "all"
	if tool == "" || tool == "all" {
		tool = "all"
	}

	tools := pickTools(tool)
	if len(tools) == 0 {
		return fmt.Errorf("no installed Python tool to configure (looked for pip/uv/poetry)")
	}

	fmt.Printf("Using mirror %s (%s) — %s\n", mirror.ID, mirror.Name, mirror.URL)
	for _, t := range tools {
		switch t {
		case "pip":
			if err := setupPip(mirror.URL, host, opts); err != nil {
				return fmt.Errorf("pip: %w", err)
			}
		case "uv":
			if err := setupUv(mirror.URL, host, opts); err != nil {
				return fmt.Errorf("uv: %w", err)
			}
		case "poetry":
			if err := setupPoetry(mirror.URL, opts); err != nil {
				return fmt.Errorf("poetry: %w", err)
			}
		}
	}
	return nil
}

func pickTools(want string) []string {
	all := []string{}
	if commandExists("pip") || commandExists("pip3") {
		all = append(all, "pip")
	}
	if commandExists("uv") {
		all = append(all, "uv")
	}
	if commandExists("poetry") {
		all = append(all, "poetry")
	}
	if want == "all" {
		return all
	}
	for _, t := range all {
		if t == want {
			return []string{t}
		}
	}
	return nil
}

// --- pip ---

func setupPip(urlStr, host string, opts adapter.Options) error {
	home, _ := os.UserHomeDir()
	primary := filepath.Join(home, ".config", "pip", "pip.conf")
	legacy := filepath.Join(home, ".pip", "pip.conf")

	dst := primary
	if fileExists(legacy) && !fileExists(primary) {
		dst = legacy
	}

	content := fmt.Sprintf(`[global]
index-url = %s
trusted-host = %s
timeout = 120
retries = 5

[install]
use-pep517 = true
`, urlStr, host)

	return writeConfig("pip", dst, []byte(content), opts)
}

// --- uv ---

func setupUv(urlStr, host string, opts adapter.Options) error {
	home, _ := os.UserHomeDir()
	dst := filepath.Join(home, ".config", "uv", "uv.toml")

	content := fmt.Sprintf(`[pip]
index-url = %q
trusted-host = [%q]
`, urlStr, host)

	if err := writeConfig("uv", dst, []byte(content), opts); err != nil {
		return err
	}
	if !opts.DryRun {
		fmt.Printf("  hint: also export UV_DEFAULT_INDEX=%s for one-off use\n", urlStr)
	}
	return nil
}

// --- poetry ---

func setupPoetry(urlStr string, opts adapter.Options) error {
	// Poetry stores config in OS-specific locations; the canonical way to
	// modify it is via the `poetry config` CLI, which the bash script also
	// used. Faithfully invoke it (or print the commands for dry-run).
	commands := [][]string{
		{"poetry", "config", "repositories.china-mirror", urlStr},
		{"poetry", "config", "http-basic.china-mirror", "", ""},
	}
	if opts.DryRun {
		fmt.Println("[dry-run] poetry would run:")
		for _, c := range commands {
			fmt.Println("  " + strings.Join(c, " "))
		}
		return nil
	}
	for _, c := range commands {
		if err := runCmd(c[0], c[1:]...); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(c, " "), err)
		}
	}
	fmt.Println("✓ poetry configured (repositories.china-mirror = " + urlStr + ")")
	return nil
}

// --- shared helpers ---

func writeConfig(tool, dst string, content []byte, opts adapter.Options) error {
	if opts.DryRun {
		fmt.Printf("[dry-run] would write %s:\n", dst)
		fmt.Println(indent(string(content), "    "))
		return nil
	}

	if fileExists(dst) {
		if !opts.Force && !opts.Yes {
			// Bash behaviour: always overwrite after backing up; no prompt
			// unless explicitly requested. We do the same.
		}
		dir, err := config.BackupFile(dst, tool)
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
	fmt.Printf("✓ %s config written: %s\n", tool, dst)
	return nil
}

func resolveMirror(store *mirrors.Store, id string) (mirrors.Mirror, error) {
	if id != "" {
		m, ok := store.Get(id)
		if !ok {
			return mirrors.Mirror{}, fmt.Errorf("unknown mirror id %q (try `cmc list mirrors --category pip`)", id)
		}
		if m.Category != "pip" {
			return mirrors.Mirror{}, fmt.Errorf("mirror %q is in category %q, not pip", id, m.Category)
		}
		return m, nil
	}
	m, ok := store.Default("pip")
	if !ok {
		return mirrors.Mirror{}, fmt.Errorf("no active pip mirror in mirrors.yml")
	}
	return m, nil
}

func hostOf(rawurl string) (string, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return "", err
	}
	return u.Host, nil
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
