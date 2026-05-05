package mobileapi

import (
	"errors"
	"net/http"
	"strings"

	"mobile_server/internal/core"
)

func (s *Server) handleStockEntryLookup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	_, ok := s.authorize(w, r)
	if !ok {
		return
	}

	query := r.URL.Query()
	barcode := strings.TrimSpace(query.Get("barcode"))
	if barcode == "" {
		barcode = strings.TrimSpace(query.Get("epc"))
	}
	if barcode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "barcode is required"})
		return
	}

	limit := parsePositiveInt(query.Get("limit"), 20)
	lookup, err := s.auth.StockEntryLookupByBarcode(r.Context(), barcode, limit)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrInvalidInput):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "barcode is required"})
		case errors.Is(err, core.ErrStockEntryNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "stock entry not found"})
		case errors.Is(err, core.ErrDirectDBLookupUnavailable):
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "direct db lookup unavailable"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stock entry lookup failed"})
		}
		return
	}

	writeJSON(w, http.StatusOK, lookup)
}
