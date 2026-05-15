package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

type Format string

const (
	Table Format = "table"
	JSON  Format = "json"
	YAML  Format = "yaml"
	MD    Format = "md"
	CSV   Format = "csv"
)

// Parse normalizes a format string. Returns Table by default.
func Parse(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return JSON
	case "yaml", "yml":
		return YAML
	case "md", "markdown":
		return MD
	case "csv":
		return CSV
	default:
		return Table
	}
}

// Row-oriented input: callers provide header + rows for table/csv/md, and
// the original typed value for json/yaml.
type Tabular struct {
	Headers []string
	Rows    [][]string
	Value   any // used for json/yaml only
}

func Render(w io.Writer, f Format, t Tabular) error {
	switch f {
	case JSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(t.Value)
	case YAML:
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		defer enc.Close()
		return enc.Encode(t.Value)
	case CSV:
		cw := csv.NewWriter(w)
		if err := cw.Write(t.Headers); err != nil {
			return err
		}
		if err := cw.WriteAll(t.Rows); err != nil {
			return err
		}
		cw.Flush()
		return cw.Error()
	case MD:
		return renderMD(w, t)
	default:
		return renderTable(w, t)
	}
}

func renderTable(w io.Writer, t Tabular) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, strings.Join(t.Headers, "\t")); err != nil {
		return err
	}
	for _, r := range t.Rows {
		if _, err := fmt.Fprintln(tw, strings.Join(r, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func renderMD(w io.Writer, t Tabular) error {
	if _, err := fmt.Fprintln(w, "| "+strings.Join(t.Headers, " | ")+" |"); err != nil {
		return err
	}
	sep := make([]string, len(t.Headers))
	for i := range sep {
		sep[i] = "---"
	}
	if _, err := fmt.Fprintln(w, "| "+strings.Join(sep, " | ")+" |"); err != nil {
		return err
	}
	for _, r := range t.Rows {
		if _, err := fmt.Fprintln(w, "| "+strings.Join(r, " | ")+" |"); err != nil {
			return err
		}
	}
	return nil
}
