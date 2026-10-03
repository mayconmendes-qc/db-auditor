package api

import (
	"net/http/httptest"
	"testing"
)

func TestParseListPage(t *testing.T) {
	for _, tc := range []struct {
		query         string
		limit, offset int
		valid         bool
	}{
		{"", 20, 0, true},
		{"?limit=100&offset=200", 100, 200, true},
		{"?limit=0", 0, 0, false},
		{"?limit=501", 0, 0, false},
		{"?offset=-1", 0, 0, false},
	} {
		limit, offset, valid := parseListPage(httptest.NewRequest("GET", "/api/v1/findings"+tc.query, nil), 20)
		if limit != tc.limit || offset != tc.offset || valid != tc.valid {
			t.Errorf("%q got %d,%d,%v", tc.query, limit, offset, valid)
		}
	}
}
