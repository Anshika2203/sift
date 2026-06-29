package ui

import "testing"

func TestParseBindingsBasic(t *testing.T) {
	b, err := parseBindings([]string{"ctrl-a:select-all,ctrl-d:deselect-all"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b["ctrl-a"]) != 1 || b["ctrl-a"][0].typ != actSelectAll {
		t.Errorf("ctrl-a = %v, want select-all", b["ctrl-a"])
	}
	if len(b["ctrl-d"]) != 1 || b["ctrl-d"][0].typ != actDeselectAll {
		t.Errorf("ctrl-d = %v, want deselect-all", b["ctrl-d"])
	}
}

func TestParseBindingsChainedActions(t *testing.T) {
	b, err := parseBindings([]string{"enter:toggle+down"})
	if err != nil {
		t.Fatal(err)
	}
	acts := b["enter"]
	if len(acts) != 2 || acts[0].typ != actToggle || acts[1].typ != actDown {
		t.Errorf("enter = %v, want [toggle down]", acts)
	}
}

func TestParseBindingsArgWithCommasAndColons(t *testing.T) {
	// commas and colons inside the parenthesised argument must not be split.
	b, err := parseBindings([]string{"ctrl-r:reload(grep -rn foo: . , bar)"})
	if err != nil {
		t.Fatal(err)
	}
	acts := b["ctrl-r"]
	if len(acts) != 1 || acts[0].typ != actReload {
		t.Fatalf("ctrl-r = %v, want one reload", acts)
	}
	if acts[0].arg != "grep -rn foo: . , bar" {
		t.Errorf("arg = %q", acts[0].arg)
	}
}

func TestParseBindingsExecuteAndBecome(t *testing.T) {
	b, err := parseBindings([]string{"ctrl-e:execute(vim {}),ctrl-o:become(code {})"})
	if err != nil {
		t.Fatal(err)
	}
	if b["ctrl-e"][0].typ != actExecute || b["ctrl-e"][0].arg != "vim {}" {
		t.Errorf("ctrl-e = %v", b["ctrl-e"])
	}
	if b["ctrl-o"][0].typ != actBecome || b["ctrl-o"][0].arg != "code {}" {
		t.Errorf("ctrl-o = %v", b["ctrl-o"])
	}
}

func TestParseBindingsKeyAliases(t *testing.T) {
	b, err := parseBindings([]string{"space:toggle, escape:abort"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := b[" "]; !ok {
		t.Error("space should normalize to ' '")
	}
	if _, ok := b["esc"]; !ok {
		t.Error("escape should normalize to 'esc'")
	}
}

func TestParseBindingsErrors(t *testing.T) {
	if _, err := parseBindings([]string{"ctrl-a"}); err == nil {
		t.Error("expected error for missing ':'")
	}
	if _, err := parseBindings([]string{"ctrl-a:bogus-action"}); err == nil {
		t.Error("expected error for unknown action")
	}
	if _, err := parseBindings([]string{"ctrl-a:reload"}); err == nil {
		t.Error("expected error for reload without argument")
	}
}
