package ansi

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestParsePlain(t *testing.T) {
	plain, spans := Parse("hello world")
	if plain != "hello world" {
		t.Errorf("plain = %q, want 'hello world'", plain)
	}
	if len(spans) != 0 {
		t.Errorf("expected no spans, got %v", spans)
	}
}

func TestParseStripsCodes(t *testing.T) {
	// red "ERROR" then reset, then plain
	plain, spans := Parse("\x1b[31mERROR\x1b[0m ok")
	if plain != "ERROR ok" {
		t.Errorf("plain = %q, want 'ERROR ok'", plain)
	}
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d: %v", len(spans), spans)
	}
	if spans[0].Start != 0 || spans[0].End != 5 {
		t.Errorf("span = [%d,%d), want [0,5)", spans[0].Start, spans[0].End)
	}
	if spans[0].Style != tcell.StyleDefault.Foreground(tcell.PaletteColor(1)) {
		t.Errorf("span style not red")
	}
}

func TestParse256AndTrueColor(t *testing.T) {
	plain, spans := Parse("\x1b[38;5;202mX\x1b[0m\x1b[38;2;10;20;30mY\x1b[0m")
	if plain != "XY" {
		t.Errorf("plain = %q, want 'XY'", plain)
	}
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
	if spans[0].Style != tcell.StyleDefault.Foreground(tcell.PaletteColor(202)) {
		t.Error("first span should be 256-color 202")
	}
	if spans[1].Style != tcell.StyleDefault.Foreground(tcell.NewRGBColor(10, 20, 30)) {
		t.Error("second span should be truecolor 10,20,30")
	}
}

func TestParseIncompleteEscape(t *testing.T) {
	// A lone ESC[ without terminator is kept literally (not dropped).
	plain, _ := Parse("a\x1b[b")
	if plain == "" {
		t.Error("expected non-empty plain text for malformed escape")
	}
}
