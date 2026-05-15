package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RestoreLatest restores the most recent backup for `tool` back to its
// original path, returning the path that was written.
func RestoreLatest(tool string) (string, error) {
	entry, ok, err := Latest(tool)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("no backups found for %q", tool)
	}
	return RestoreEntry(entry)
}

// RestoreByID restores a specific backup identified by tool + backup ID.
func RestoreByID(tool, id string) (string, error) {
	dir := filepath.Join(BackupDir(), tool, id)
	metaPath := filepath.Join(dir, "metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", metaPath, err)
	}
	var md Metadata
	if err := json.Unmarshal(data, &md); err != nil {
		return "", fmt.Errorf("parse %s: %w", metaPath, err)
	}
	entry := Entry{
		Tool:         md.Tool,
		BackupID:     md.BackupID,
		OriginalPath: md.OriginalPath,
		BackupPath:   filepath.Join(dir, filepath.Base(md.OriginalPath)),
		Time:         md.BackupTime,
	}
	return RestoreEntry(entry)
}

// RestoreEntry copies `entry.BackupPath` back over `entry.OriginalPath`.
// The destination directory is created if missing.
func RestoreEntry(entry Entry) (string, error) {
	if _, err := os.Stat(entry.BackupPath); err != nil {
		return "", fmt.Errorf("backup file missing: %s", entry.BackupPath)
	}
	if err := os.MkdirAll(filepath.Dir(entry.OriginalPath), 0o755); err != nil {
		return "", err
	}
	if _, err := copyFile(entry.BackupPath, entry.OriginalPath); err != nil {
		return "", err
	}
	return entry.OriginalPath, nil
}
