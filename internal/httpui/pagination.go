package httpui

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Onellan/leanPrice/internal/storage"
)

var errInvalidPageCursor = errors.New("invalid page cursor")

func validatePageCursor(r *http.Request, path string) error {
	switch path {
	case "/products", "/search":
		name, id := r.URL.Query().Get("after_name"), r.URL.Query().Get("after_id")
		if len(name) > 240 || len(id) > storage.MaxProductCursorIDSize || (name == "") != (id == "") || len(r.URL.Query().Get("q")) > 120 {
			return errInvalidPageCursor
		}
	case "/activity":
		_, _, err := activityPageCursor(r)
		return err
	}
	return nil
}

func productPageCursor(r *http.Request) (string, string) {
	return r.URL.Query().Get("after_name"), r.URL.Query().Get("after_id")
}

func productNextPageURL(path, query string, product storage.ProductSummary) string {
	values := url.Values{}
	if path == "/search" && query != "" {
		values.Set("q", query)
	}
	values.Set("after_name", product.SortName)
	values.Set("after_id", product.ID)
	return path + "?" + values.Encode()
}

func activityPageCursor(r *http.Request) (*storage.ActivityCursor, bool, error) {
	values := r.URL.Query()
	atText, key := strings.TrimSpace(values.Get("after_at")), values.Get("after_key")
	if atText == "" && key == "" {
		return nil, false, nil
	}
	at, err := strconv.ParseInt(atText, 10, 64)
	if err != nil || at <= 0 || key == "" || len(key) > 120 {
		return nil, false, errInvalidPageCursor
	}
	return &storage.ActivityCursor{At: at, EventKey: key}, true, nil
}

func activityNextPageURL(item storage.ActivityItem) string {
	values := url.Values{}
	values.Set("after_at", strconv.FormatInt(item.At, 10))
	values.Set("after_key", item.EventKey)
	return "/activity?" + values.Encode()
}

func historyPageCursor(r *http.Request) (*storage.HistoryCursor, bool, error) {
	values := r.URL.Query()
	atText, observationID, offerID := strings.TrimSpace(values.Get("after_at")), values.Get("after_observation"), values.Get("after_offer")
	if atText == "" && observationID == "" && offerID == "" {
		return nil, false, nil
	}
	at, err := strconv.ParseInt(atText, 10, 64)
	if err != nil || at <= 0 || observationID == "" || len(observationID) > 80 || len(offerID) > 80 {
		return nil, false, errInvalidPageCursor
	}
	return &storage.HistoryCursor{ObservedAt: at, ObservationID: observationID, OfferID: offerID}, true, nil
}

func historyNextPageURL(productID string, item storage.HistorySummary) string {
	values := url.Values{}
	values.Set("after_at", strconv.FormatInt(item.ObservedAt, 10))
	values.Set("after_observation", item.ObservationID)
	values.Set("after_offer", item.OfferID)
	return "/products/" + url.PathEscape(productID) + "?" + values.Encode()
}
