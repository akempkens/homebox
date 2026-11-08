package customsinventory

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validRequest() Request {
	return Request{
		Scope:   Scope{ItemIDs: []uuid.UUID{uuid.New()}},
		Options: Options{GroupBy: GroupByLocation, ReferenceDate: "2026-11-15"},
	}
}

func TestRequestValidate(t *testing.T) {
	require.NoError(t, validRequest().Validate())

	ok := map[string]func(r *Request){
		"nil receipt types": func(r *Request) { r.Options.ReceiptTypes = nil },
		"min months 0":      func(r *Request) { r.Options.MinMonths = 0 },
		"min months 60":     func(r *Request) { r.Options.MinMonths = 60 },
		"query scope":       func(r *Request) { r.Scope = Scope{Query: &ScopeQuery{}} },
		"tag grouping":      func(r *Request) { r.Options.GroupBy = GroupByTag; r.Options.GroupTagIDs = []uuid.UUID{uuid.New()} },
		"all receipt types": func(r *Request) { r.Options.ReceiptTypes = AllowedReceiptTypes },
	}
	for name, mutate := range ok {
		r := validRequest()
		mutate(&r)
		require.NoError(t, r.Validate(), name)
	}

	bad := map[string]func(r *Request){
		"empty reference date":   func(r *Request) { r.Options.ReferenceDate = "" },
		"invalid reference date": func(r *Request) { r.Options.ReferenceDate = "2026-13-01" },
		"min months -1":          func(r *Request) { r.Options.MinMonths = -1 },
		"min months 61":          func(r *Request) { r.Options.MinMonths = 61 },
		"ids and query":          func(r *Request) { r.Scope.Query = &ScopeQuery{} },
		"neither ids nor query":  func(r *Request) { r.Scope = Scope{} },
		"unknown group by":       func(r *Request) { r.Options.GroupBy = "room" },
		"tag without group tags": func(r *Request) { r.Options.GroupBy = GroupByTag },
		"photo receipt type":     func(r *Request) { r.Options.ReceiptTypes = []string{"photo"} },
	}
	for name, mutate := range bad {
		r := validRequest()
		mutate(&r)
		assert.ErrorIs(t, r.Validate(), ErrInvalidRequest, name)
	}
}

func TestItem_SoldBy(t *testing.T) {
	ref := d("2026-11-15")
	cases := []struct {
		name string
		item Item
		want bool
	}{
		{"neither", Item{}, false},
		{"date before", Item{SoldDate: d("2026-01-10")}, true},
		{"date on reference", Item{SoldDate: d("2026-11-15")}, true},
		{"date after", Item{SoldDate: d("2026-11-16")}, false},
		{"date before, with price", Item{SoldDate: d("2026-01-10"), SoldPrice: 45.00}, true},
		{"date after, with price", Item{SoldDate: d("2026-11-16"), SoldPrice: 45.00}, false},
		{"price only", Item{SoldPrice: 45.00}, true},
		{"rounds to 0 cents", Item{SoldPrice: 0.004}, false},
		{"rounds to 1 cent", Item{SoldPrice: 0.005}, true},
		{"negative", Item{SoldPrice: -5.00}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.item.SoldBy(ref))
		})
	}
}
