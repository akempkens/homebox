package customsinventory

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMarkup(t *testing.T) {
	p := func(s string) span { return span{text: s} }
	b := func(s string) span { return span{text: s, bold: true} }
	i := func(s string) span { return span{text: s, italic: true} }
	u := func(s string) span { return span{text: s, underline: true} }
	bi := func(s string) span { return span{text: s, bold: true, italic: true} }

	for _, tc := range []struct {
		in   string
		want []span
	}{
		{"", nil},
		{"plain text", []span{p("plain text")}},
		{"**Oak** case", []span{b("Oak"), p(" case")}},
		{"__Oak__, made", []span{b("Oak"), p(", made")}},
		{"an *old* clock", []span{p("an "), i("old"), p(" clock")}},
		{"an _old_ clock", []span{p("an "), i("old"), p(" clock")}},
		{"***both***", []span{bi("both")}},
		{"___both___", []span{bi("both")}},
		{"<u>1890</u> model", []span{u("1890"), p(" model")}},
		{"<U>1890</U>", []span{u("1890")}},
		{"**bold _and italic_**", []span{b("bold "), bi("and italic")}},
		{"**a** and **b**", []span{b("a"), p(" and "), b("b")}},
		{"[link](http://x) **b**", []span{p("[link](http://x) "), b("b")}},
		{"snake_case_name", []span{p("snake_case_name")}},
		{"2*3*4", []span{p("2*3*4")}},
		{"2 * 3 * 4", []span{p("2 * 3 * 4")}},
		{"* list item", []span{p("* list item")}},
		{"**unclosed", []span{p("**unclosed")}},
		{"**x*", []span{p("**x*")}},
		{"\\*not italic\\*", []span{p("\\*not italic\\*")}},
		{"****x****", []span{p("****x****")}},
		{"<u>open only", []span{p("<u>open only")}},
		{"**a _b** c_", []span{b("a _b"), p(" c_")}},
		{"\\<u>x</u>", []span{p("\\<u>x</u>")}},
		{"<u>a\\</u>", []span{p("<u>a\\</u>")}},
	} {
		assert.Equal(t, tc.want, parseMarkup(tc.in), "%q", tc.in)
	}
}
