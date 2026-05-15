// Package config backs up and restores the configuration files that
// the adapters touch. The on-disk format is intentionally identical
// to common.sh:backup_file in china-mirror-skills so backups produced
// by the legacy bash scripts can still be restored by `cmc restore`
// during the migration window.
package config

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupDir is where backups are written. Override CHINA_MIRROR_BACKUP_DIR
// to relocate (mostly useful in tests).
func BackupDir() string {
	if v := os.Getenv("CHINA_MIRROR_BACKUP_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".china-mirror-backup")
}

// Metadata mirrors the JSON written by common.sh:backup_file.
type Metadata struct {
	OriginalPath string `json:"original_path"`
	BackupTime   string `json:"backup_time"`
	Tool         string `json:"tool"`
	BackupID     string `json:"backup_id"`
	FileSize     int64  `json:"file_size"`
	Checksum     string `json:"checksum"`
}

// Entry is a single backed-up file as seen from `cmc backup --list` etc.
type Entry struct {
	Tool         string
	BackupID     string
	OriginalPath string
	BackupPath   string
	Time         string
}

// newBackupID matches the bash format `date +%Y%m%d_%H%M%S`.
func newBackupID() string {
	return time.Now().Format("20060102_150405")
}

// BackupFile copies `path` into BackupDir/<tool>/<id>/ and writes a
// metadata.json next to it. Returns the directory the backup landed in.
// Missing files are not an error — they just produce no backup entry.
func BackupFile(path, tool string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, expected file", path)
	}

	id := newBackupID()
	dir := filepath.Join(BackupDir(), tool, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	dst := filepath.Join(dir, filepath.Base(path))
	sum, err := copyFile(path, dst)
	if err != nil {
		return "", err
	}

	meta := Metadata{
		OriginalPath: path,
		BackupTime:   time.Now().Format(time.RFC3339),
		Tool:         tool,
		BackupID:     id,
		FileSize:     info.Size(),
		Checksum:     sum,
	}
	if err := writeMetadata(dir, meta); err != nil {
		return "", err
	}
	return dir, nil
}

// List returns every backup entry under BackupDir, newest first. When
// `tool` is empty, every tool is returned; otherwise the listing is
// scoped to that tool.
func List(tool string) ([]Entry, error) {
	root := BackupDir()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	}

	pattern := filepath.Join(root, "*", "*", "metadata.json")
	if tool != "" {
		pattern = filepath.Join(root, tool, "*", "metadata.json")
	}
	metas, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	out := make([]Entry, 0, len(metas))
	for _, m := range metas {
		var md Metadata
		data, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(data, &md); err != nil {
			continue
		}
		dir := filepath.Dir(m)
		out = append(out, Entry{
			Tool:         md.Tool,
			BackupID:     md.BackupID,
			OriginalPath: md.OriginalPath,
			BackupPath:   filepath.Join(dir, filepath.Base(md.OriginalPath)),
			Time:         md.BackupTime,
		})
	}

	// Newest first: backup IDs are timestamps, so a reverse string sort works.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Tool != out[j].Tool {
			return out[i].Tool < out[j].Tool
		}
		return out[i].BackupID > out[j].BackupID
	})
	return out, nil
}

// Latest returns the newest backup for a tool, or false if none exist.
func Latest(tool string) (Entry, bool, error) {
	entries, err := List(tool)
	if err != nil {
		return Entry{}, false, err
	}
	for _, e := range entries {
		if e.Tool == tool {
			return e, true, nil
		}
	}
	return Entry{}, false, nil
}

// --- helpers ---

func copyFile(src, dst string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return "", err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return "", err
	}
	h := md5.New()
	if _, err := io.Copy(io.MultiWriter(out, h), in); err != nil {
		out.Close()
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeMetadata(dir string, m Metadata) error {
	data, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "metadata.json"), data, 0o644)
}

// SanitizeChecksum trims/normalizes a checksum string for display.
func SanitizeChecksum(s string) string { return strings.TrimSpace(s) }
