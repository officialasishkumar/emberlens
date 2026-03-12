package display

import (
	"fmt"
	"io"
	"strings"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiCyan   = "\033[36m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
)

// Printer handles formatted terminal output with optional ANSI color support.
type Printer struct {
	W     io.Writer
	Color bool
}

func (p *Printer) wrap(code, s string) string {
	if !p.Color {
		return s
	}
	return code + s + ansiReset
}

func (p *Printer) Bold(s string) string   { return p.wrap(ansiBold, s) }
func (p *Printer) Dim(s string) string    { return p.wrap(ansiDim, s) }
func (p *Printer) Cyan(s string) string   { return p.wrap(ansiCyan, s) }
func (p *Printer) Green(s string) string  { return p.wrap(ansiBold+ansiGreen, s) }
func (p *Printer) Yellow(s string) string { return p.wrap(ansiBold+ansiYellow, s) }

// Banner prints a styled title bar with an underline separator.
func (p *Printer) Banner(parts ...string) {
	title := strings.Join(parts, " · ")
	width := 0
	for range title {
		width++
	}
	fmt.Fprintf(p.W, "\n  %s\n", p.Bold(title))
	fmt.Fprintf(p.W, "  %s\n\n", p.Dim(strings.Repeat("\u2500", width)))
}

// Table renders aligned columns with a bold header and dim separator lines.
func (p *Printer) Table(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i := 0; i < len(row) && i < len(widths); i++ {
			if len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
	}

	var hdr, sep strings.Builder
	hdr.WriteString("  ")
	sep.WriteString("  ")
	for i, h := range headers {
		fmt.Fprintf(&hdr, "%-*s", widths[i], h)
		sep.WriteString(strings.Repeat("\u2500", widths[i]))
		if i < len(headers)-1 {
			hdr.WriteString("  ")
			sep.WriteString("  ")
		}
	}
	fmt.Fprintln(p.W, p.Bold(hdr.String()))
	fmt.Fprintln(p.W, p.Dim(sep.String()))

	for _, row := range rows {
		var line strings.Builder
		line.WriteString("  ")
		for i := 0; i < len(headers); i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			fmt.Fprintf(&line, "%-*s", widths[i], cell)
			if i < len(headers)-1 {
				line.WriteString("  ")
			}
		}
		fmt.Fprintln(p.W, line.String())
	}

	fmt.Fprintln(p.W, p.Dim(sep.String()))
}

// CardField is a key-value pair for card-style output.
type CardField struct {
	Label string
	Value string
}

// Card prints a single entry in a vertical card layout.
func (p *Printer) Card(number int, fields []CardField) {
	header := fmt.Sprintf("\u2500\u2500 #%d ", number)
	pad := 42 - len(header)
	if pad < 4 {
		pad = 4
	}
	header += strings.Repeat("\u2500", pad)
	fmt.Fprintf(p.W, "  %s\n", p.Dim(header))
	for _, f := range fields {
		label := f.Label + ":"
		if len(label) < 18 {
			label += strings.Repeat(" ", 18-len(label))
		}
		fmt.Fprintf(p.W, "  %s%s\n", p.Cyan(label), f.Value)
	}
	fmt.Fprintln(p.W)
}

// Footer prints dim summary lines below the main output.
func (p *Printer) Footer(lines ...string) {
	fmt.Fprintln(p.W)
	for _, line := range lines {
		fmt.Fprintf(p.W, "  %s\n", p.Dim(line))
	}
	fmt.Fprintln(p.W)
}
