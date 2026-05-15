// Package flutter exports FLUTTER_STORAGE_BASE_URL and PUB_HOSTED_URL in
// the user's shell profile so the Flutter SDK and Dart pub fetch from a
// Chinese mirror.
package flutter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/loredunk/china-mirror/internal/adapter"
	"github.com/loredunk/china-mirror/internal/config"
	"github.com/loredunk/china-mirror/internal/mirrors"
)

func init() { adapter.Register(&Adapter{}) }

const (
	blockMarker = "# china-mirror-cli: flutter"
	blockEnd    = "# end china-mirror-cli: flutter"
)

// pubHosted maps a flutter mirror ID to its companion pub.dev mirror.
// flutter-cfug pairs storage.flutter-io.cn with pub.flutter-io.cn.
var pubHosted = map[string]string{
	"flutter-cfug": "https://pub.flutter-io.cn",
}

type Adapter struct{}

func (a *Adapter) Name() string         { return "flutter" }
func (a *Adapter) Categories() []string { return []string{"flutter"} }
func (a *Adapter) Description() string {
	return "Set FLUTTER_STORAGE_BASE_URL / PUB_HOSTED_URL to Chinese mirrors"
}
func (a *Adapter) BackupTargets() []string { return config.ToolConfigs["flutter"] }
func (a *Adapter) Commands() []adapter.Command {
	return []adapter.Command{
		{Name: "setup", Description: "Append Flutter mirror exports to the active shell profile"},
	}
}

func (a *Adapter) Run(cmd string, opts adapter.Options) error {
	switch cmd {
	case "setup":
		return a.setup(opts)
	}
	return fmt.Errorf("flutter: unknown command %q", cmd)
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
	storageURL := strings.TrimRight(mirror.URL, "/")
	pubURL, ok := pubHosted[mirror.ID]
	if !ok {
		// Fallback: assume same host with pub. prefix swap.
		pubURL = storageURL
	}

	block := fmt.Sprintf(`%s
export FLUTTER_STORAGE_BASE_URL="%s"
export PUB_HOSTED_URL="%s"
%s
`, blockMarker, storageURL, pubURL, blockEnd)

	fmt.Printf("Using mirror %s (%s)\n", mirror.ID, mirror.Name)
	fmt.Printf("  FLUTTER_STORAGE_BASE_URL=%s\n", storageURL)
	fmt.Printf("  PUB_HOSTED_URL=%s\n", pubURL)

	profile, err := pickShellProfile()
	if err != nil {
		return err
	}
	return writeProfileBlock(profile, block, opts)
}

func resolveMirror(store *mirrors.Store, id string) (mirrors.Mirror, error) {
	if id != "" {
		m, ok := store.Get(id)
		if !ok {
			return mirrors.Mirror{}, fmt.Errorf("unknown mirror id %q (try `cmc list mirrors --category flutter`)", id)
		}
		if m.Category != "flutter" {
			return mirrors.Mirror{}, fmt.Errorf("mirror %q is in category %q, not flutter", id, m.Category)
		}
		return m, nil
	}
	m, ok := store.Default("flutter")
	if !ok {
		return mirrors.Mirror{}, fmt.Errorf("no active flutter mirror in mirrors.yml")
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
		dir, err := config.BackupFile(path, "flutter")
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
	fmt.Printf("✓ flutter env appended to %s\n", path)
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

