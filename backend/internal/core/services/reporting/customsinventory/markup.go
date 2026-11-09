package customsinventory

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// span is a piece of one description line with its formatting (FR-50).
type span struct {
	text                    string
	bold, italic, underline bool
}

type markKind int

const (
	markText   markKind = iota
	markDelim           // a run of 1–3 '*' or '_'
	markUOpen           // <u>
	markUClose          // </u>
)

type mark struct {
	kind              markKind
	text              string // the source text; equal text = same character and length
	canOpen, canClose bool
	matched, opens    bool // part of a pair; the pair's opening marker
}

// parseMarkup reads bold, italic and underline markers in one line of a
// description (FR-50). Matched markers are removed; everything else is kept
// as written. Adjacent spans with the same formatting are merged.
func parseMarkup(line string) []span {
	marks := tokenizeMarkup(line)
	matchMarks(marks)
	return markSpans(marks)
}

func tokenizeMarkup(line string) []mark {
	var marks []mark
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			marks = append(marks, mark{kind: markText, text: text.String()})
			text.Reset()
		}
	}
	for i := 0; i < len(line); {
		switch {
		case hasPrefixFold(line[i:], "<u>"):
			flush()
			marks = append(marks, mark{kind: markUOpen, text: line[i : i+3]})
			i += 3
		case hasPrefixFold(line[i:], "</u>"):
			flush()
			marks = append(marks, mark{kind: markUClose, text: line[i : i+4]})
			i += 4
		case line[i] == '*' || line[i] == '_':
			j := i
			for j < len(line) && line[j] == line[i] {
				j++
			}
			if j-i > 3 || (i > 0 && line[i-1] == '\\') {
				text.WriteString(line[i:j]) // plain text
			} else {
				flush()
				marks = append(marks, delimMark(line, i, j))
			}
			i = j
		default:
			text.WriteByte(line[i])
			i++
		}
	}
	flush()
	return marks
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// delimMark builds the marker for the run line[i:j]. It can open when it's
// followed by a non-space and not preceded by a letter or digit, and close
// when it's preceded by a non-space and not followed by a letter or digit.
func delimMark(line string, i, j int) mark {
	prev, _ := utf8.DecodeLastRuneInString(line[:i])
	next, _ := utf8.DecodeRuneInString(line[j:])
	prevWord := i > 0 && isWordRune(prev)
	nextWord := j < len(line) && isWordRune(next)
	return mark{
		kind:     markDelim,
		text:     line[i:j],
		canOpen:  j < len(line) && !unicode.IsSpace(next) && !prevWord,
		canClose: i > 0 && !unicode.IsSpace(prev) && !nextWord,
	}
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// matchMarks pairs closers with the nearest open marker of the same kind.
// Open markers above a match stay unmatched, so they are plain text.
func matchMarks(marks []mark) {
	var stack []int
	pair := func(s, k int) {
		marks[stack[s]].matched, marks[stack[s]].opens = true, true
		marks[k].matched = true
		stack = stack[:s]
	}
	for k := range marks {
		m := marks[k]
		switch m.kind {
		case markDelim:
			if m.canClose {
				if s := findOpen(marks, stack, func(o mark) bool { return o.kind == markDelim && o.text == m.text }); s >= 0 {
					pair(s, k)
					continue
				}
			}
			if m.canOpen {
				stack = append(stack, k)
			}
		case markText:
		case markUOpen:
			stack = append(stack, k)
		case markUClose:
			if s := findOpen(marks, stack, func(o mark) bool { return o.kind == markUOpen }); s >= 0 {
				pair(s, k)
			}
		}
	}
}

// findOpen returns the stack position of the topmost open marker that fits, or -1.
func findOpen(marks []mark, stack []int, fits func(mark) bool) int {
	for s := len(stack) - 1; s >= 0; s-- {
		if fits(marks[stack[s]]) {
			return s
		}
	}
	return -1
}

// markSpans turns the marks into spans. Matched pairs nest (the stack
// guarantees it), so depth counters give each piece its formatting.
func markSpans(marks []mark) []span {
	var bold, italic, underline int
	var out []span
	for _, m := range marks {
		if m.matched {
			d := -1
			if m.opens {
				d = 1
			}
			if m.kind == markDelim {
				if len(m.text) >= 2 {
					bold += d
				}
				if len(m.text) != 2 {
					italic += d
				}
			} else {
				underline += d
			}
			continue
		}
		out = appendSpan(out, span{text: m.text, bold: bold > 0, italic: italic > 0, underline: underline > 0})
	}
	return out
}

func appendSpan(out []span, s span) []span {
	if s.text == "" {
		return out
	}
	if n := len(out); n > 0 && out[n-1].bold == s.bold && out[n-1].italic == s.italic && out[n-1].underline == s.underline {
		out[n-1].text += s.text
		return out
	}
	return append(out, s)
}
