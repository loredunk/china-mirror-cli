package config

import (
	"os"
	"path/filepath"
)

// ToolConfigs is the single source of truth for which files belong to
// which tool — used by `cmc backup <tool>` to back up everything an
// adapter might touch, even when called outside the adapter itself.
// Matches the TOOL_CONFIGS map in scripts/backup_config.sh.
var ToolConfigs = map[string][]string{
	"pip":      pathsHome(".config/pip/pip.conf", ".pip/pip.conf"),
	"uv":       pathsHome(".config/uv/uv.toml"),
	"poetry":   pathsHome(".config/pypoetry/config.toml", "Library/Application Support/pypoetry/config.toml"),
	"npm":      pathsHome(".npmrc"),
	"yarn":     pathsHome(".yarnrc", ".yarnrc.yml"),
	"pnpm":     pathsHome(".npmrc"),
	"docker":   {"/etc/docker/daemon.json"},
	"apt":      {"/etc/apt/sources.list"},
	"homebrew": pathsHome(".bash_profile", ".zshrc", ".config/fish/config.fish"),
	"conda":    pathsHome(".condarc"),
	"cargo":    pathsHome(".cargo/config.toml", ".cargo/config"),
	"go":       pathsHome(".bash_profile", ".zshrc"),
	"flutter":  pathsHome(".bash_profile", ".zshrc"),
	"github":   pathsHome(".gitconfig"),
	"gem":      pathsHome(".gemrc"),
}

// KnownTools returns the sorted set of tool keys ToolConfigs knows about.
func KnownTools() []string {
	out := make([]string, 0, len(ToolConfigs))
	for k := range ToolConfigs {
		out = append(out, k)
	}
	return out
}

// ExistingConfigs filters ToolConfigs[tool] down to the files that
// actually exist on disk right now.
func ExistingConfigs(tool string) []string {
	out := []string{}
	for _, p := range ToolConfigs[tool] {
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func pathsHome(parts ...string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return parts
	}
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = filepath.Join(home, p)
	}
	return out
}
