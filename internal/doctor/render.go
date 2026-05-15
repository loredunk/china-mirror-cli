package doctor

import (
	"fmt"
	"io"
	"strings"
)

// RenderText prints a human-readable Markdown-flavoured summary of the
// report. Used for the default `--format table` output of cmc doctor.
func (r *Report) RenderText(w io.Writer) error {
	bw := &bufWriter{w: w}
	bw.line("# Doctor report")
	bw.line("Generated: " + r.Generated.Format("2006-01-02 15:04:05 UTC"))
	bw.line("")

	bw.line("## System")
	bw.line(fmt.Sprintf("  platform: %s (%s/%s)", r.OS.Platform, r.OS.GOOS, r.OS.GOARCH))
	if r.OS.Distro != "" {
		bw.line("  distro:   " + r.OS.Distro)
	}
	bw.line("  shell:    " + r.OS.Shell)
	bw.line("")

	bw.line("## Proxy")
	if len(r.Proxy.EnvVars) == 0 && r.Proxy.GitProxy == "" {
		bw.line("  ✓ no proxy environment variables set")
	} else {
		for k, v := range r.Proxy.EnvVars {
			bw.line(fmt.Sprintf("  ⚠ %s=%s", k, v))
		}
		if r.Proxy.GitProxy != "" {
			bw.line("  ⚠ git http.proxy=" + r.Proxy.GitProxy)
		}
		bw.line("  hint: unset before configuring mirrors")
	}
	bw.line("")

	bw.line("## Tools")
	for _, t := range r.Tools {
		icon := "✗"
		if t.Installed {
			if t.UsingChinaMirror {
				icon = "✓"
			} else {
				icon = "○"
			}
		}
		row := fmt.Sprintf("  %s %s", icon, t.Name)
		if t.Version != "" {
			row += " — " + t.Version
		}
		bw.line(row)
		if t.Installed && t.MirrorConfig != "" {
			bw.line("      " + truncate(t.MirrorConfig, 100))
		}
	}
	bw.line("")

	bw.line("## Connectivity")
	for _, p := range r.Connectivity {
		icon := "✗"
		if p.OK {
			icon = "✓"
		}
		extra := ""
		if p.HTTP != 0 {
			extra = fmt.Sprintf(" (HTTP %d, %.2fs)", p.HTTP, p.LatencyS)
		} else if p.Error != "" {
			extra = " — " + truncate(p.Error, 60)
		}
		bw.line(fmt.Sprintf("  %s %-22s %s%s", icon, p.Label, p.URL, extra))
	}
	bw.line("")

	bw.line("## Recommendations")
	if len(r.Recommendations) == 0 {
		bw.line("  ✓ none")
	}
	for _, rec := range r.Recommendations {
		sev := strings.ToUpper(string(rec.Severity))
		prefix := ""
		switch rec.Severity {
		case SevHigh:
			prefix = "🔴 HIGH"
		case SevMedium:
			prefix = "🟡 MED"
		case SevOK:
			prefix = "✅ OK"
		default:
			prefix = sev
		}
		bw.line("  " + prefix + " " + rec.Message)
	}

	return bw.err
}

// --- helpers ---

type bufWriter struct {
	w   io.Writer
	err error
}

func (b *bufWriter) line(s string) {
	if b.err != nil {
		return
	}
	_, b.err = fmt.Fprintln(b.w, s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
