// Package output provides CLI output utilities with color support.
package output

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ansiRegex matches ANSI escape codes
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// VisibleWidth returns the visible width of a string (excluding ANSI codes)
func VisibleWidth(s string) int {
	// Strip ANSI codes
	clean := ansiRegex.ReplaceAllString(s, "")
	return utf8.RuneCountInString(clean)
}

// Table handles aligned table output with ANSI color support.
type Table struct {
	out     io.Writer
	headers []string
	rows    [][]string
	padding int
}

// NewTable creates a new Table instance.
func NewTable() *Table {
	return &Table{
		out:     os.Stdout,
		padding: 2,
	}
}

// SetOutput sets the output writer.
func (t *Table) SetOutput(w io.Writer) *Table {
	t.out = w
	return t
}

// SetPadding sets the column padding.
func (t *Table) SetPadding(p int) *Table {
	t.padding = p
	return t
}

// SetHeaders sets the table headers.
func (t *Table) SetHeaders(headers ...string) *Table {
	t.headers = headers
	return t
}

// AddRow adds a row to the table.
func (t *Table) AddRow(cells ...string) *Table {
	t.rows = append(t.rows, cells)
	return t
}

// Render outputs the table with proper alignment.
func (t *Table) Render() {
	if len(t.headers) == 0 && len(t.rows) == 0 {
		return
	}

	// Calculate column widths based on visible text
	colCount := len(t.headers)
	if colCount == 0 && len(t.rows) > 0 {
		colCount = len(t.rows[0])
	}

	widths := make([]int, colCount)

	// Check header widths
	for i, h := range t.headers {
		if i < colCount {
			w := VisibleWidth(h)
			if w > widths[i] {
				widths[i] = w
			}
		}
	}

	// Check row widths
	for _, row := range t.rows {
		for i, cell := range row {
			if i < colCount {
				w := VisibleWidth(cell)
				if w > widths[i] {
					widths[i] = w
				}
			}
		}
	}

	// Print headers
	if len(t.headers) > 0 {
		t.printRow(t.headers, widths)
		// Print separator
		sep := make([]string, colCount)
		for i, w := range widths {
			sep[i] = strings.Repeat("-", w)
		}
		t.printRow(sep, widths)
	}

	// Print rows
	for _, row := range t.rows {
		t.printRow(row, widths)
	}
}

// printRow prints a single row with proper alignment.
func (t *Table) printRow(cells []string, widths []int) {
	for i, cell := range cells {
		if i >= len(widths) {
			break
		}

		// Calculate padding needed
		visWidth := VisibleWidth(cell)
		padNeeded := widths[i] - visWidth

		if padNeeded < 0 {
			padNeeded = 0
		}

		// Print cell with padding
		fmt.Fprint(t.out, cell)
		fmt.Fprint(t.out, strings.Repeat(" ", padNeeded))

		// Add column separator (except for last column)
		if i < len(cells)-1 {
			fmt.Fprint(t.out, strings.Repeat(" ", t.padding))
		}
	}
	fmt.Fprintln(t.out)
}
