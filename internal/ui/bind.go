package ui

import (
	"fmt"
	"strings"
)

// actionType enumerates the things a key or event binding can do.
type actionType int

const (
	actIgnore actionType = iota
	actUp
	actDown
	actPageUp
	actPageDown
	actHalfPageUp
	actHalfPageDown
	actFirst
	actLast
	actAccept
	actAbort
	actToggle
	actToggleAll
	actSelectAll
	actDeselectAll
	actClearQuery
	actClearSelection
	actBackwardDeleteChar
	actTogglePreview
	actPreviewUp
	actPreviewDown
	actPreviewPageUp
	actPreviewPageDown
	actChangeQuery
	actChangePrompt
	actPut
	actExecute
	actExecuteSilent
	actBecome
	actReload
)

type action struct {
	typ actionType
	arg string
}

// actionSpec describes a recognised action name.
type actionSpec struct {
	typ      actionType
	takesArg bool
}

var actionNames = map[string]actionSpec{
	"ignore":               {actIgnore, false},
	"up":                   {actUp, false},
	"down":                 {actDown, false},
	"page-up":              {actPageUp, false},
	"page-down":            {actPageDown, false},
	"half-page-up":         {actHalfPageUp, false},
	"half-page-down":       {actHalfPageDown, false},
	"first":                {actFirst, false},
	"top":                  {actFirst, false},
	"last":                 {actLast, false},
	"accept":               {actAccept, false},
	"abort":                {actAbort, false},
	"cancel":               {actAbort, false},
	"toggle":               {actToggle, false},
	"toggle-all":           {actToggleAll, false},
	"select-all":           {actSelectAll, false},
	"deselect-all":         {actDeselectAll, false},
	"clear-query":          {actClearQuery, false},
	"clear-selection":      {actClearSelection, false},
	"backward-delete-char": {actBackwardDeleteChar, false},
	"toggle-preview":       {actTogglePreview, false},
	"preview-up":           {actPreviewUp, false},
	"preview-down":         {actPreviewDown, false},
	"preview-page-up":      {actPreviewPageUp, false},
	"preview-page-down":    {actPreviewPageDown, false},
	"change-query":         {actChangeQuery, true},
	"change-prompt":        {actChangePrompt, true},
	"put":                  {actPut, true},
	"execute":              {actExecute, true},
	"execute-silent":       {actExecuteSilent, true},
	"become":               {actBecome, true},
	"reload":               {actReload, true},
}

// parseBindings parses one or more --bind specs into a key/event -> actions map.
// A spec is a comma-separated list of "KEY:ACTION[+ACTION...]" entries, where an
// action may carry a parenthesised argument, e.g. "ctrl-r:reload(ls)".
func parseBindings(specs []string) (map[string][]action, error) {
	out := map[string][]action{}
	for _, spec := range specs {
		for _, pair := range splitTop(spec, ',') {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			ci := topIndex(pair, ':')
			if ci < 0 {
				return nil, fmt.Errorf("invalid --bind %q (expected KEY:ACTION)", pair)
			}
			key := normalizeKey(strings.TrimSpace(pair[:ci]))
			var acts []action
			for _, tok := range splitTop(pair[ci+1:], '+') {
				if strings.TrimSpace(tok) == "" {
					continue
				}
				a, err := parseAction(tok)
				if err != nil {
					return nil, err
				}
				acts = append(acts, a)
			}
			out[key] = acts
		}
	}
	return out, nil
}

func parseAction(tok string) (action, error) {
	tok = strings.TrimSpace(tok)
	name, arg, hasArg := tok, "", false
	if i := strings.IndexByte(tok, '('); i >= 0 && strings.HasSuffix(tok, ")") {
		name, arg, hasArg = tok[:i], tok[i+1:len(tok)-1], true
	}
	name = strings.ToLower(strings.TrimSpace(name))
	spec, ok := actionNames[name]
	if !ok {
		return action{}, fmt.Errorf("unknown action: %q", name)
	}
	if spec.takesArg && !hasArg {
		return action{}, fmt.Errorf("action %q requires an argument, e.g. %s(...)", name, name)
	}
	return action{typ: spec.typ, arg: arg}, nil
}

// normalizeKey canonicalises a key name to match keyName().
func normalizeKey(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	switch k {
	case "space":
		return " "
	case "bspace", "backspace":
		return "bspace"
	case "return":
		return "enter"
	case "escape":
		return "esc"
	}
	return k
}

// splitTop splits s on sep, ignoring separators nested inside parentheses.
func splitTop(s string, sep byte) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case sep:
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// topIndex returns the index of the first sep at paren depth 0, or -1.
func topIndex(s string, sep byte) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case sep:
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
