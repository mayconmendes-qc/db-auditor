package api

import (
	"net/http"
	"strconv"
	"strings"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 500
)

// PageMeta is the standard pagination envelope fragment.
type PageMeta struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

// PageResponse is the standard list envelope.
type PageResponse struct {
	Items any      `json:"items"`
	Page  PageMeta `json:"page"`
}

// InventoryQuery holds common inventory list filters.
type InventoryQuery struct {
	Limit    int
	Offset   int
	Q        string
	Database string
	Schema   string
	OrderBy  string
}

func parseInventoryQuery(r *http.Request) InventoryQuery {
	q := InventoryQuery{
		Limit:    defaultPageLimit,
		Offset:   0,
		Q:        strings.TrimSpace(r.URL.Query().Get("q")),
		Database: strings.TrimSpace(r.URL.Query().Get("database")),
		Schema:   strings.TrimSpace(r.URL.Query().Get("schema")),
		OrderBy:  strings.TrimSpace(r.URL.Query().Get("order_by")),
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			q.Limit = n
		}
	}
	if q.Limit > maxPageLimit {
		q.Limit = maxPageLimit
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			q.Offset = n
		}
	}
	return q
}

func pageMeta(limit, offset, total int) PageMeta {
	return PageMeta{
		Limit:   limit,
		Offset:  offset,
		Total:   total,
		HasMore: offset+limit < total,
	}
}

func writePage(w http.ResponseWriter, items any, limit, offset, total int) {
	if items == nil {
		items = []any{}
	}
	writeJSON(w, http.StatusOK, PageResponse{
		Items: items,
		Page:  pageMeta(limit, offset, total),
	})
}
