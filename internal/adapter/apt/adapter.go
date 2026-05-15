// Package apt rewrites /etc/apt/sources.list for Ubuntu/Debian to use a
// Chinese mirror. Requires root or sudo; registers itself on import.
package apt

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/loredunk/china-mirror/internal/adapter"
	"github.com/loredunk/china-mirror/internal/config"
	"github.com/loredunk/china-mirror/internal/mirrors"
)

func init() { adapter.Register(&Adapter{}) }

const sourcesList = "/etc/apt/sources.list"

type Adapter struct{}

func (a *Adapter) Name() string         { return "apt" }
func (a *Adapter) Categories() []string { return []string{"ubuntu"} }
func (a *Adapter) Description() string {
	return "Rewrite /etc/apt/sources.list for Ubuntu/Debian to a Chinese mirror"
}
func (a *Adapter) BackupTargets() []string { return config.ToolConfigs["apt"] }
func (a *Adapter) Commands() []adapter.Command {
	return []adapter.Command{
		{Name: "setup", Description: "Rewrite /etc/apt/sources.list (requires root or sudo)"},
	}
}

func (a *Adapter) Run(cmd string, opts adapter.Options) error {
	switch cmd {
	case "setup":
		return a.setup(opts)
	}
	return fmt.Errorf("apt: unknown command %q", cmd)
}

func (a *Adapter) setup(opts adapter.Options) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("apt is only supported on Linux")
	}

	store, err := mirrors.Load()
	if err != nil {
		return fmt.Errorf("load mirrors: %w", err)
	}
	mirror, err := resolveMirror(store, opts.Mirror)
	if err != nil {
		return err
	}

	distro, err := detectDistro()
	if err != nil {
		return err
	}
	if distro != "ubuntu" && distro != "debian" {
		return fmt.Errorf("unsupported distribution %q (expected ubuntu/debian)", distro)
	}

	codename := opts.Extra["codename"]
	if codename == "" {
		codename, err = detectCodename()
		if err != nil {
			return fmt.Errorf("auto-detect codename failed (use --extra codename=jammy): %w", err)
		}
	}

	fmt.Printf("Using mirror %s (%s) — %s\n", mirror.ID, mirror.Name, mirror.URL)
	fmt.Printf("Distribution: %s, codename: %s\n", distro, codename)

	content := buildSources(distro, codename, strings.TrimSuffix(mirror.URL, "/"))

	if opts.DryRun {
		fmt.Println("[dry-run] would write " + sourcesList + ":")
		fmt.Println(indent(content, "    "))
		return nil
	}

	if !isRoot() {
		return fmt.Errorf("writing %s requires root (run with sudo)", sourcesList)
	}

	if fileExists(sourcesList) {
		dir, err := config.BackupFile(sourcesList, "apt")
		if err != nil {
			return fmt.Errorf("backup %s: %w", sourcesList, err)
		}
		if dir != "" {
			fmt.Printf("  backed up %s → %s\n", sourcesList, dir)
		}
	}
	if err := os.WriteFile(sourcesList, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Printf("✓ apt config written: %s\n", sourcesList)

	if !opts.Yes && !opts.Force {
		fmt.Println("  hint: run `sudo apt-get update` to refresh the package list")
		return nil
	}
	return runCmd("apt-get", "update", "-qq")
}

// buildSources generates a sources.list body. Ubuntu strips the trailing
// "ubuntu/" if present in the mirror URL (we add it back), Debian uses
// the "debian/" / "debian-security/" pair.
func buildSources(distro, codename, base string) string {
	switch distro {
	case "ubuntu":
		// mirror URL already points at .../ubuntu/ for ubuntu-* mirrors;
		// strip a trailing /ubuntu to normalise.
		root := strings.TrimSuffix(base, "/ubuntu")
		return fmt.Sprintf(`# Ubuntu APT sources configured by china-mirror-cli
# Mirror: %s/ubuntu/

deb %s/ubuntu/ %s main restricted universe multiverse
deb %s/ubuntu/ %s-updates main restricted universe multiverse
deb %s/ubuntu/ %s-backports main restricted universe multiverse
deb %s/ubuntu/ %s-security main restricted universe multiverse
`, root, root, codename, root, codename, root, codename, root, codename)
	case "debian":
		root := strings.TrimSuffix(base, "/debian")
		return fmt.Sprintf(`# Debian APT sources configured by china-mirror-cli
# Mirror: %s

deb %s/debian/ %s main contrib non-free
deb %s/debian/ %s-updates main contrib non-free
deb %s/debian-security/ %s-security main contrib non-free
`, root, root, codename, root, codename, root, codename)
	}
	return ""
}

func resolveMirror(store *mirrors.Store, id string) (mirrors.Mirror, error) {
	if id != "" {
		m, ok := store.Get(id)
		if !ok {
			return mirrors.Mirror{}, fmt.Errorf("unknown mirror id %q (try `cmc list mirrors --category ubuntu`)", id)
		}
		if m.Category != "ubuntu" {
			return mirrors.Mirror{}, fmt.Errorf("mirror %q is in category %q, not ubuntu", id, m.Category)
		}
		return m, nil
	}
	m, ok := store.Default("ubuntu")
	if !ok {
		return mirrors.Mirror{}, fmt.Errorf("no active ubuntu mirror in mirrors.yml")
	}
	return m, nil
}

func detectDistro() (string, error) {
	v, err := readOSRelease("ID")
	if err != nil {
		return "", err
	}
	return strings.ToLower(v), nil
}

func detectCodename() (string, error) {
	return readOSRelease("VERSION_CODENAME")
}

func readOSRelease(key string) (string, error) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, key+"=") {
			return strings.Trim(strings.TrimPrefix(line, key+"="), `"`), nil
		}
	}
	return "", fmt.Errorf("%s not set in /etc/os-release", key)
}

func isRoot() bool { return os.Geteuid() == 0 }

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
