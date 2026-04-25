package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestVisibleWidth(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"plain ASCII", "hello", 5},
		{"plain empty", "", 0},
		{"with ANSI color", "\x1b[32mgreen\x1b[0m", 5},
		{"multi-byte runes counted once", "héllo", 5},
		{"mixed ANSI and runes", "\x1b[1mhéllo\x1b[0m world", 11},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := VisibleWidth(tc.in); got != tc.want {
				t.Errorf("VisibleWidth(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestTable_RenderEmpty(t *testing.T) {
	var buf bytes.Buffer
	tb := NewTable().SetOutput(&buf)
	tb.Render() // headers=nil, rows=nil → early return
	if buf.Len() != 0 {
		t.Errorf("empty table rendered something: %q", buf.String())
	}
}

func TestTable_RenderBasic(t *testing.T) {
	var buf bytes.Buffer
	tb := NewTable().SetOutput(&buf)
	tb.SetHeaders("NAME", "VERSION")
	tb.AddRow("aider", "0.86.1")
	tb.AddRow("claude-code", "2.1.3")
	tb.Render()

	out := buf.String()
	for _, s := range []string{"NAME", "VERSION", "aider", "0.86.1", "claude-code", "2.1.3"} {
		if !strings.Contains(out, s) {
			t.Errorf("rendered output missing %q\n---\n%s", s, out)
		}
	}

	// Column count: each non-empty line should contain both columns.
	// Every non-empty row should contain both columns on the same line —
	// the shorter row should still have spaces between its first cell and
	// the second column, proving the padding pass ran.
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		if !strings.Contains(line, "  ") {
			t.Errorf("line %q lacks inter-column padding", line)
		}
	}
}

func TestTable_AlignmentWithANSIHeaders(t *testing.T) {
	var buf bytes.Buffer
	tb := NewTable().SetOutput(&buf)
	// Header with ANSI should not bloat the column width.
	tb.SetHeaders("\x1b[1mNAME\x1b[0m", "VERSION")
	tb.AddRow("longlonglong", "1.0.0")
	tb.Render()

	// Build a table WITHOUT ANSI for comparison; visible widths should match.
	var buf2 bytes.Buffer
	tb2 := NewTable().SetOutput(&buf2)
	tb2.SetHeaders("NAME", "VERSION")
	tb2.AddRow("longlonglong", "1.0.0")
	tb2.Render()

	// Lines count must match (no extra blank lines).
	got1 := strings.Count(buf.String(), "\n")
	got2 := strings.Count(buf2.String(), "\n")
	if got1 != got2 {
		t.Errorf("ANSI vs plain line counts differ: %d vs %d", got1, got2)
	}
}

func TestTable_CustomPadding(t *testing.T) {
	var buf bytes.Buffer
	tb := NewTable().SetOutput(&buf).SetPadding(5)
	tb.SetHeaders("A", "B")
	tb.AddRow("x", "y")
	tb.Render()

	// With padding=5 we expect more spaces between columns than the default.
	var def bytes.Buffer
	tbDefault := NewTable().SetOutput(&def)
	tbDefault.SetHeaders("A", "B")
	tbDefault.AddRow("x", "y")
	tbDefault.Render()

	if len(buf.String()) <= len(def.String()) {
		t.Errorf("expected wider padding to produce longer output: got %d <= default %d",
			len(buf.String()), len(def.String()))
	}
}

func TestTable_RowsWithoutHeaders(t *testing.T) {
	var buf bytes.Buffer
	tb := NewTable().SetOutput(&buf)
	// No headers, only rows — Render must still work (colCount from first row).
	tb.AddRow("a", "b", "c")
	tb.AddRow("d", "e", "f")
	tb.Render()

	out := buf.String()
	for _, s := range []string{"a", "b", "c", "d", "e", "f"} {
		if !strings.Contains(out, s) {
			t.Errorf("output missing %q\n---\n%s", s, out)
		}
	}
}
