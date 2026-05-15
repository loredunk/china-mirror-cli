// Package github accelerates GitHub access from China. It supports two
// independent modes:
//
//   - release URL conversion: rewrite https://github.com/owner/repo/...
//     into a mirror URL (TUNA/USTC path-rewrite or ghfast prefix-proxy).
//   - global clone acceleration: set `git config --global url.<proxy>.insteadOf`
//     so every `git clone https://github.com/...` runs through the proxy.
package github

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/loredunk/china-mirror/internal/adapter"
	"github.com/loredunk/china-mirror/internal/config"
	"github.com/loredunk/china-mirror/internal/mirrors"
)

func init() { adapter.Register(&Adapter{}) }

type Adapter struct{}

func (a *Adapter) Name() string         { return "github" }
func (a *Adapter) Categories() []string { return []string{"github-release", "github-repo"} }
func (a *Adapter) Description() string {
	return "Convert GitHub release URLs to mirrors and/or accelerate `git clone`"
}
func (a *Adapter) BackupTargets() []string { return config.ToolConfigs["github"] }
func (a *Adapter) Commands() []adapter.Command {
	return []adapter.Command{
		{Name: "setup", Description: "Configure git insteadOf or rewrite a single --release-url"},
	}
}

func (a *Adapter) Run(cmd string, opts adapter.Options) error {
	switch cmd {
	case "setup":
		return a.setup(opts)
	}
	return fmt.Errorf("github: unknown command %q", cmd)
}

func (a *Adapter) setup(opts adapter.Options) error {
	releaseURL := opts.Extra["release-url"]
	proxyClone := opts.Extra["proxy-clone"] == "true" || opts.Extra["proxy-clone"] == "1"

	if releaseURL == "" && !proxyClone {
		return fmt.Errorf("github: pass --extra release-url=<url> or --extra proxy-clone=true")
	}

	store, err := mirrors.Load()
	if err != nil {
		return fmt.Errorf("load mirrors: %w", err)
	}

	if releaseURL != "" {
		mirror, err := resolveMirror(store, opts.Mirror, "github-release")
		if err != nil {
			return err
		}
		rewritten, err := convertReleaseURL(releaseURL, mirror)
		if err != nil {
			return err
		}
		fmt.Println(rewritten)
		return nil
	}

	mirror, err := resolveMirror(store, opts.Mirror, "github-repo")
	if err != nil {
		return err
	}
	if mirror.Verify.Type != "github_proxy" {
		return fmt.Errorf("mirror %q does not support clone proxy (verify.type != github_proxy)", mirror.ID)
	}
	return configureProxyClone(mirror, opts)
}

// resolveMirror prefers an explicit --mirror, falls back to the default
// active mirror in the requested category.
func resolveMirror(store *mirrors.Store, id, category string) (mirrors.Mirror, error) {
	if id != "" {
		m, ok := store.Get(id)
		if !ok {
			return mirrors.Mirror{}, fmt.Errorf("unknown mirror id %q (try `cmc list mirrors --category %s`)", id, category)
		}
		return m, nil
	}
	m, ok := store.Default(category)
	if !ok {
		return mirrors.Mirror{}, fmt.Errorf("no active %s mirror in mirrors.yml", category)
	}
	return m, nil
}

var releasePathRE = regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+)/releases/download/([^/]+)/(.+)$`)

func convertReleaseURL(release string, mirror mirrors.Mirror) (string, error) {
	prefix := strings.TrimRight(mirror.URL, "/") + "/"
	if mirror.Verify.Type == "github_proxy" {
		if !strings.HasPrefix(release, "https://github.com/") {
			return "", fmt.Errorf("unsupported URL for proxy %q: %s", mirror.ID, release)
		}
		return strings.TrimRight(mirror.URL, "/") + "/" + release, nil
	}
	m := releasePathRE.FindStringSubmatch(release)
	if m == nil {
		return "", fmt.Errorf("unsupported release URL: %s\n  expected: https://github.com/<owner>/<repo>/releases/download/<tag>/<asset>", release)
	}
	owner, repo, tag, asset := m[1], m[2], m[3], m[4]
	return fmt.Sprintf("%s%s/%s/releases/download/%s/%s", prefix, owner, repo, tag, asset), nil
}

func configureProxyClone(mirror mirrors.Mirror, opts adapter.Options) error {
	if !commandExists("git") {
		return fmt.Errorf("git is required to configure clone proxy")
	}
	proxyPrefix := strings.TrimRight(mirror.URL, "/") + "/"
	insteadOf := proxyPrefix + "https://github.com/"
	gitKey := fmt.Sprintf("url.%s.insteadOf", insteadOf)

	if opts.DryRun {
		fmt.Println("[dry-run] would back up ~/.gitconfig if present")
		fmt.Println("[dry-run] would run:")
		fmt.Printf("  git config --global %q %q\n", gitKey, "https://github.com/")
		return nil
	}

	home, _ := os.UserHomeDir()
	if home != "" {
		gc := home + "/.gitconfig"
		if fileExists(gc) {
			if dir, err := config.BackupFile(gc, "github"); err == nil && dir != "" {
				fmt.Printf("  backed up %s → %s\n", gc, dir)
			}
		}
	}

	// Drop any previous matching entry, then set the new one.
	_ = exec.Command("git", "config", "--global", "--unset-all", gitKey).Run()
	if err := exec.Command("git", "config", "--global", gitKey, "https://github.com/").Run(); err != nil {
		return fmt.Errorf("git config: %w", err)
	}
	fmt.Printf("✓ configured global clone acceleration via %s\n", mirror.ID)
	fmt.Printf("  every https://github.com/ URL will be rewritten to %s\n", insteadOf)
	fmt.Printf("  to remove: git config --global --unset-all %q\n", gitKey)
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
