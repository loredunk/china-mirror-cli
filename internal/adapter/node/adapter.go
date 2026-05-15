// Package node configures npm, yarn and pnpm to use a Chinese npm
// registry mirror. Registers itself on import.
package node

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

// binaryMirrors maps a known mirror ID to a base URL for native-binary
// downloads (node/electron/sass/...). Sourced from the bash
// BINARY_MIRRORS map in node/setup.sh.
var binaryMirrors = map[string]string{
	"npm-npmmirror": "https://npmmirror.com/mirrors",
	"npm-tencent":   "https://mirrors.cloud.tencent.com",
}

type Adapter struct{}

func (a *Adapter) Name() string         { return "node" }
func (a *Adapter) Categories() []string { return []string{"npm"} }
func (a *Adapter) Description() string {
	return "Configure npm / yarn / pnpm to use a Chinese npm registry mirror"
}
func (a *Adapter) BackupTargets() []string {
	return append(append(config.ToolConfigs["npm"], config.ToolConfigs["yarn"]...), config.ToolConfigs["pnpm"]...)
}
func (a *Adapter) Commands() []adapter.Command {
	return []adapter.Command{
		{Name: "setup", Description: "Point installed Node.js package managers at a China mirror"},
	}
}

func (a *Adapter) Run(cmd string, opts adapter.Options) error {
	switch cmd {
	case "setup":
		return a.setup(opts)
	}
	return fmt.Errorf("node: unknown command %q", cmd)
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
	binBase := binaryMirrors[mirror.ID] // optional

	tool := opts.Extra["tool"]
	if tool == "" || tool == "all" {
		tool = "all"
	}
	tools := pickTools(tool)
	if len(tools) == 0 {
		return fmt.Errorf("no installed Node tool to configure (looked for npm/yarn/pnpm)")
	}

	fmt.Printf("Using mirror %s (%s) — %s\n", mirror.ID, mirror.Name, mirror.URL)
	for _, t := range tools {
		switch t {
		case "npm":
			if err := setupNpm(mirror, binBase, opts); err != nil {
				return fmt.Errorf("npm: %w", err)
			}
		case "yarn":
			if err := setupYarn(mirror.URL, opts); err != nil {
				return fmt.Errorf("yarn: %w", err)
			}
		case "pnpm":
			if err := setupPnpm(mirror.URL, opts); err != nil {
				return fmt.Errorf("pnpm: %w", err)
			}
		}
	}
	return nil
}

func pickTools(want string) []string {
	all := []string{}
	if commandExists("npm") {
		all = append(all, "npm")
	}
	if commandExists("yarn") {
		all = append(all, "yarn")
	}
	if commandExists("pnpm") {
		all = append(all, "pnpm")
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

// --- npm ---

func setupNpm(m mirrors.Mirror, binBase string, opts adapter.Options) error {
	home, _ := os.UserHomeDir()
	dst := filepath.Join(home, ".npmrc")

	var b strings.Builder
	fmt.Fprintf(&b, "# Configured by china-mirror-cli\n")
	fmt.Fprintf(&b, "# Mirror: %s (%s)\n", m.ID, m.Name)
	fmt.Fprintf(&b, "registry=%s\n\n", m.URL)
	fmt.Fprintln(&b, "fetch-retries=5")
	fmt.Fprintln(&b, "fetch-timeout=120000")
	if binBase != "" {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "# Binary mirrors for native packages")
		fmt.Fprintf(&b, "disturl=%s/node\n", binBase)
		fmt.Fprintf(&b, "electron_mirror=%s/electron/\n", binBase)
		fmt.Fprintf(&b, "chromedriver_cdnurl=%s/chromedriver\n", binBase)
		fmt.Fprintf(&b, "operadriver_cdnurl=%s/operadriver\n", binBase)
		fmt.Fprintf(&b, "phantomjs_cdnurl=%s/phantomjs\n", binBase)
		fmt.Fprintf(&b, "sass_binary_site=%s/node-sass\n", binBase)
	}
	return writeConfig("npm", dst, []byte(b.String()), opts)
}

// --- yarn / pnpm: rely on the tool's own CLI so we don't fight its config layout ---

func setupYarn(urlStr string, opts adapter.Options) error {
	cmds := [][]string{
		{"yarn", "config", "set", "registry", urlStr},
		{"yarn", "config", "set", "network-timeout", "120000"},
	}
	return execAll("yarn", cmds, opts)
}

func setupPnpm(urlStr string, opts adapter.Options) error {
	cmds := [][]string{
		{"pnpm", "config", "set", "registry", urlStr},
		{"pnpm", "config", "set", "fetch-retries", "5"},
	}
	return execAll("pnpm", cmds, opts)
}

// --- shared helpers ---

func execAll(tool string, cmds [][]string, opts adapter.Options) error {
	if opts.DryRun {
		fmt.Printf("[dry-run] %s would run:\n", tool)
		for _, c := range cmds {
			fmt.Println("  " + strings.Join(c, " "))
		}
		return nil
	}
	for _, c := range cmds {
		if err := runCmd(c[0], c[1:]...); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(c, " "), err)
		}
	}
	fmt.Printf("✓ %s configured\n", tool)
	return nil
}

func writeConfig(tool, dst string, content []byte, opts adapter.Options) error {
	if opts.DryRun {
		fmt.Printf("[dry-run] would write %s:\n", dst)
		fmt.Println(indent(string(content), "    "))
		return nil
	}
	if fileExists(dst) {
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
			return mirrors.Mirror{}, fmt.Errorf("unknown mirror id %q (try `cmc list mirrors --category npm`)", id)
		}
		if m.Category != "npm" {
			return mirrors.Mirror{}, fmt.Errorf("mirror %q is in category %q, not npm", id, m.Category)
		}
		return m, nil
	}
	m, ok := store.Default("npm")
	if !ok {
		return mirrors.Mirror{}, fmt.Errorf("no active npm mirror in mirrors.yml")
	}
	return m, nil
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
