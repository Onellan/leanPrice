package httpui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Onellan/leanPrice/internal/auth"
	"github.com/Onellan/leanPrice/internal/storage"
)

func TestHomeRendersFullPageAndHTMXFragment(t *testing.T) {
	router, credentials, _, _, _ := newHTTPTest(t, true)

	full := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodGet, "http://example.test/", nil, credentials)
	router.ServeHTTP(full, request)
	if full.Code != http.StatusOK {
		t.Fatalf("full page status = %d, want 200", full.Code)
	}
	if !strings.Contains(full.Body.String(), "<!doctype html>") || !strings.Contains(full.Body.String(), "<nav") {
		t.Fatalf("full page is missing its document shell or navigation: %s", full.Body.String())
	}
	if !strings.Contains(full.Header().Get("Content-Security-Policy"), "'self'") {
		t.Fatalf("Content-Security-Policy = %q, want local-only policy", full.Header().Get("Content-Security-Policy"))
	}
	if !strings.Contains(full.Body.String(), `name="csrf_token"`) {
		t.Fatal("authenticated page did not render the sign-out CSRF token")
	}

	fragment := httptest.NewRecorder()
	request = authenticatedRequest(http.MethodGet, "http://example.test/", nil, credentials)
	request.Header.Set("HX-Request", "true")
	router.ServeHTTP(fragment, request)
	if fragment.Code != http.StatusOK {
		t.Fatalf("fragment status = %d, want 200", fragment.Code)
	}
	if strings.Contains(fragment.Body.String(), "<!doctype html>") || !strings.Contains(fragment.Body.String(), "Your price watch") {
		t.Fatalf("HTMX request did not receive the page fragment: %s", fragment.Body.String())
	}
}

func TestProductCatalogPaginationKeepsStableCursorAndRejectsMalformedCursor(t *testing.T) {
	router, credentials, _, store, _ := newHTTPTest(t, true)
	for i := 0; i < storage.InteractivePageSize+1; i++ {
		if err := store.CreateCanonicalProduct(context.Background(), storage.ProductSummary{
			ID: fmt.Sprintf("page-%03d", i), Name: fmt.Sprintf("Product %03d", i), Quantity: "1/1", QuantityUnit: "item", PackCount: 1,
		}, time.Unix(int64(i+1), 0)); err != nil {
			t.Fatal(err)
		}
	}
	first := httptest.NewRecorder()
	router.ServeHTTP(first, authenticatedRequest(http.MethodGet, "http://example.test/products", nil, credentials))
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), "Next products") || !strings.Contains(first.Body.String(), "Product 049") || strings.Contains(first.Body.String(), "Product 050") {
		t.Fatalf("first product page status=%d body=%s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	router.ServeHTTP(second, authenticatedRequest(http.MethodGet, "http://example.test/products?after_name=product+049&after_id=page-049", nil, credentials))
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), "Product 050") || strings.Contains(second.Body.String(), "Product 049") {
		t.Fatalf("second product page status=%d body=%s", second.Code, second.Body.String())
	}
	malformed := httptest.NewRecorder()
	router.ServeHTTP(malformed, authenticatedRequest(http.MethodGet, "http://example.test/products?after_name=product+049", nil, credentials))
	if malformed.Code != http.StatusBadRequest {
		t.Fatalf("malformed cursor status = %d, want 400", malformed.Code)
	}
}

func TestPriceChartUsesBoundedIntegerCoordinatesAndTextLabels(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	samples := []storage.ProductPriceSample{
		{ObservedAt: start.UnixMilli(), Retailer: "Clicks", AmountMinor: 2499, Currency: "ZAR", Scale: 2},
		{ObservedAt: start.Add(24 * time.Hour).UnixMilli(), Retailer: "Makro", AmountMinor: 1999, Currency: "ZAR", Scale: 2},
	}
	points := buildPriceChart(samples)
	if len(points) != 2 || points[0].X != 20 || points[1].X != 300 || points[0].Y >= points[1].Y || !strings.Contains(points[0].AccessibleLabel, "Clicks · R24.99") {
		t.Fatalf("price chart points = %+v", points)
	}
	if len(buildPriceChart(nil)) != 0 {
		t.Fatal("empty sample unexpectedly produced chart points")
	}
}

func TestPriceDisplayKeepsForeignCurrencySeparateFromRand(t *testing.T) {
	if got := displayPrice(1999, "USD", 2); got != "USD 19.99" {
		t.Fatalf("foreign currency display = %q", got)
	}
	if got := displayPrice(1999, "ZAR", 2); got != "R19.99" {
		t.Fatalf("rand display = %q", got)
	}
}

func TestSettingsShowsCurrentApplicationCgroupAndPauseThreshold(t *testing.T) {
	_, credentials, service, store, _ := newHTTPTest(t, true)
	router, err := NewRouter(Options{
		Ready: func() bool { return true }, Auth: service, Store: store,
		MemoryStatus: func(context.Context) (ResourceStatus, error) {
			return ResourceStatus{CurrentBytes: 123_456_789, SoftLimit: 350_000_000, ResumeAt: 320_000_000, Paused: true}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, authenticatedRequest(http.MethodGet, "http://example.test/settings", nil, credentials))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "123.5 MB decimal (123456789 bytes)") || !strings.Contains(body, "paused while memory pressure clears") || !strings.Contains(body, "does not measure total Raspberry Pi use") {
		t.Fatalf("settings memory panel status=%d body=%s", response.Code, body)
	}
}

func TestWatchNotificationPreferenceAndVersionedEdit(t *testing.T) {
	_, credentials, service, store, _ := newHTTPTest(t, true)
	product := storage.ProductSummary{ID: "watch-edit-product", Name: "Watch edit product", Quantity: "1/1", QuantityUnit: "item", PackCount: 1}
	if err := store.CreateCanonicalProduct(context.Background(), product, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	router, err := NewRouter(Options{
		Ready: func() bool { return true }, Auth: service, Store: store,
		TestNotification: func(context.Context, string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	get := httptest.NewRecorder()
	router.ServeHTTP(get, authenticatedRequest(http.MethodGet, "http://example.test/watches/new?product="+product.ID, nil, credentials))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `value="webhook"`) || !strings.Contains(get.Body.String(), "Current offers for Watch edit product") || !strings.Contains(get.Body.String(), "No alert is evaluated or sent by this preview") {
		t.Fatalf("configured webhook option missing: %d %s", get.Code, get.Body.String())
	}
	createForm := url.Values{"product_id": {product.ID}, "target_price": {"R10.00"}, "notification_channel": {"webhook"}, csrfFormField: {credentials.CSRF}}
	create := authenticatedRequest(http.MethodPost, "http://example.test/watches/new", strings.NewReader(createForm.Encode()), credentials)
	create.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	create.Header.Set("Origin", "http://example.test")
	created := httptest.NewRecorder()
	router.ServeHTTP(created, create)
	if created.Code != http.StatusSeeOther {
		t.Fatalf("create webhook watch = %d %s", created.Code, created.Body.String())
	}
	watches, err := store.ListWatches(context.Background(), 10)
	if err != nil || len(watches) != 1 || watches[0].NotificationChannel != "webhook" {
		t.Fatalf("saved notification preference = %+v, %v", watches, err)
	}
	editURL := "http://example.test/watches/" + watches[0].ID + "/edit"
	editPage := httptest.NewRecorder()
	router.ServeHTTP(editPage, authenticatedRequest(http.MethodGet, editURL, nil, credentials))
	if editPage.Code != http.StatusOK || !strings.Contains(editPage.Body.String(), `name="expected_rule_version" value="1"`) || !strings.Contains(editPage.Body.String(), "Save watch changes") {
		t.Fatalf("watch edit page = %d %s", editPage.Code, editPage.Body.String())
	}
	editForm := url.Values{"expected_rule_version": {"1"}, "product_id": {product.ID}, "target_price": {"R9.99"},
		"notification_channel": {"in_app"}, csrfFormField: {credentials.CSRF}}
	edit := authenticatedRequest(http.MethodPost, editURL, strings.NewReader(editForm.Encode()), credentials)
	edit.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	edit.Header.Set("Origin", "http://example.test")
	updated := httptest.NewRecorder()
	router.ServeHTTP(updated, edit)
	if updated.Code != http.StatusSeeOther {
		t.Fatalf("update watch rule = %d %s", updated.Code, updated.Body.String())
	}
	watch, err := store.WatchByID(context.Background(), watches[0].ID)
	if err != nil || watch.RuleVersion != 2 || watch.NotificationChannel != "in_app" || !watch.TargetMinor.Valid || watch.TargetMinor.Int64 != 999 {
		t.Fatalf("updated watch = %+v, %v", watch, err)
	}
	staleForm := url.Values{"expected_rule_version": {"1"}, "product_id": {product.ID}, "target_price": {"R8.00"},
		"notification_channel": {"in_app"}, csrfFormField: {credentials.CSRF}}
	stale := authenticatedRequest(http.MethodPost, editURL, strings.NewReader(staleForm.Encode()), credentials)
	stale.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	stale.Header.Set("Origin", "http://example.test")
	conflict := httptest.NewRecorder()
	router.ServeHTTP(conflict, stale)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("stale watch update = %d %s", conflict.Code, conflict.Body.String())
	}
}

func TestTestNotificationIsExplicitAuthenticatedCSRFProtectedAndRedacted(t *testing.T) {
	_, credentials, service, store, _ := newHTTPTest(t, true)
	var callbackErr error
	calls := 0
	router, err := NewRouter(Options{
		Ready: func() bool { return true }, Auth: service, Store: store, RequestTimeout: time.Second,
		TestNotification: func(_ context.Context, key string) error {
			calls++
			if key == "" {
				t.Error("test notification did not receive an idempotency key")
			}
			return callbackErr
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	settings := httptest.NewRecorder()
	router.ServeHTTP(settings, authenticatedRequest(http.MethodGet, "http://example.test/settings", nil, credentials))
	if settings.Code != http.StatusOK || !strings.Contains(settings.Body.String(), "Send test notification") {
		t.Fatalf("configured notification test is not visible: %d %s", settings.Code, settings.Body.String())
	}

	badCSRF := httptest.NewRecorder()
	badRequest := authenticatedRequest(http.MethodPost, "http://example.test/settings/notifications/test", strings.NewReader(""), credentials)
	badRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badRequest.Header.Set("Origin", "http://example.test")
	router.ServeHTTP(badCSRF, badRequest)
	if badCSRF.Code != http.StatusForbidden || calls != 0 {
		t.Fatalf("invalid-CSRF test notification = %d; callback calls=%d", badCSRF.Code, calls)
	}

	post := func() *httptest.ResponseRecorder {
		form := url.Values{csrfFormField: {credentials.CSRF}}
		request := authenticatedRequest(http.MethodPost, "http://example.test/settings/notifications/test", strings.NewReader(form.Encode()), credentials)
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("Origin", "http://example.test")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	success := post()
	if success.Code != http.StatusOK || !strings.Contains(success.Body.String(), "Test notification sent") || calls != 1 {
		t.Fatalf("valid test notification = %d; callback calls=%d; body=%s", success.Code, calls, success.Body.String())
	}
	callbackErr = errors.New("secret webhook token leaked from transport")
	failure := post()
	if failure.Code != http.StatusServiceUnavailable || strings.Contains(failure.Body.String(), "secret webhook token") || !strings.Contains(failure.Body.String(), "could not be delivered") {
		t.Fatalf("failed test notification was not safely redacted: %d %s", failure.Code, failure.Body.String())
	}
}

func TestProductAndWatchFormsPersistExactLocalState(t *testing.T) {
	router, credentials, _, store, _ := newHTTPTest(t, true)
	productFormPage := httptest.NewRecorder()
	router.ServeHTTP(productFormPage, authenticatedRequest(http.MethodGet, "http://example.test/products/new", nil, credentials))
	if productFormPage.Code != http.StatusOK || strings.Contains(productFormPage.Body.String(), "Saved product variants") {
		t.Fatalf("product form route rendered the catalog beneath the form: %d %s", productFormPage.Code, productFormPage.Body.String())
	}
	form := url.Values{
		"name":        {"Laundry liquid 750 ml"},
		"brand":       {"Example"},
		"line":        {"Laundry"},
		"variant":     {"Fresh"},
		"quantity":    {"750 ml"},
		"gtin":        {""},
		csrfFormField: {credentials.CSRF},
	}
	request := authenticatedRequest(http.MethodPost, "http://example.test/products/new", strings.NewReader(form.Encode()), credentials)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.test")
	created := httptest.NewRecorder()
	router.ServeHTTP(created, request)
	if created.Code != http.StatusSeeOther || created.Header().Get("Location") != "/products" {
		t.Fatalf("create product = %d, location %q, body %s", created.Code, created.Header().Get("Location"), created.Body.String())
	}
	products, err := store.ListProducts(context.Background(), 10)
	if err != nil || len(products) != 1 || products[0].Quantity != "3/4 L" || products[0].PackCount != 1 {
		t.Fatalf("saved products = %+v, %v", products, err)
	}
	watchFormPage := httptest.NewRecorder()
	router.ServeHTTP(watchFormPage, authenticatedRequest(http.MethodGet, "http://example.test/watches/new?product="+products[0].ID, nil, credentials))
	if watchFormPage.Code != http.StatusOK || strings.Contains(watchFormPage.Body.String(), "Price watches") {
		t.Fatalf("watch form route rendered the watch list beneath the form: %d %s", watchFormPage.Code, watchFormPage.Body.String())
	}
	watchForm := url.Values{"product_id": {products[0].ID}, "target_price": {"R49.99"}, csrfFormField: {credentials.CSRF}}
	watchRequest := authenticatedRequest(http.MethodPost, "http://example.test/watches/new", strings.NewReader(watchForm.Encode()), credentials)
	watchRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	watchRequest.Header.Set("Origin", "http://example.test")
	watchResponse := httptest.NewRecorder()
	router.ServeHTTP(watchResponse, watchRequest)
	if watchResponse.Code != http.StatusSeeOther || watchResponse.Header().Get("Location") != "/watches" {
		t.Fatalf("create watch = %d, location %q, body %s", watchResponse.Code, watchResponse.Header().Get("Location"), watchResponse.Body.String())
	}
	watches, err := store.ListWatches(context.Background(), 10)
	if err != nil || len(watches) != 1 || !watches[0].Active || !watches[0].TargetMinor.Valid || watches[0].TargetMinor.Int64 != 4999 {
		t.Fatalf("saved watches = %+v, %v", watches, err)
	}
	pauseForm := url.Values{"active": {"false"}, csrfFormField: {credentials.CSRF}}
	pauseRequest := authenticatedRequest(http.MethodPost, "http://example.test/watches/"+watches[0].ID+"/active", strings.NewReader(pauseForm.Encode()), credentials)
	pauseRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	pauseRequest.Header.Set("Origin", "http://example.test")
	paused := httptest.NewRecorder()
	router.ServeHTTP(paused, pauseRequest)
	if paused.Code != http.StatusSeeOther {
		t.Fatalf("pause watch = %d, body %s", paused.Code, paused.Body.String())
	}
	watches, err = store.ListWatches(context.Background(), 10)
	if err != nil || len(watches) != 1 || watches[0].Active {
		t.Fatalf("watch pause state = %+v, %v", watches, err)
	}
	relativeForm := url.Values{"product_id": {products[0].ID}, "drop_percentage": {"12"}, csrfFormField: {credentials.CSRF}}
	relativeRequest := authenticatedRequest(http.MethodPost, "http://example.test/watches/new", strings.NewReader(relativeForm.Encode()), credentials)
	relativeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	relativeRequest.Header.Set("Origin", "http://example.test")
	relativeResponse := httptest.NewRecorder()
	router.ServeHTTP(relativeResponse, relativeRequest)
	if relativeResponse.Code != http.StatusSeeOther || relativeResponse.Header().Get("Location") != "/watches" {
		t.Fatalf("create relative-drop watch = %d, location %q, body %s", relativeResponse.Code, relativeResponse.Header().Get("Location"), relativeResponse.Body.String())
	}
	watches, err = store.ListWatches(context.Background(), 10)
	if err != nil || len(watches) != 2 {
		t.Fatalf("watches after relative-drop create = %+v, %v", watches, err)
	}
	foundRelative := false
	for _, watch := range watches {
		if watch.DropBPS.Valid && watch.DropBPS.Int64 == 1200 && !watch.TargetMinor.Valid && watch.ConditionOperator == "any" {
			foundRelative = true
		}
	}
	if !foundRelative {
		t.Fatalf("relative-drop rule was not stored in basis points: %+v", watches)
	}
}

func TestProductIdentityCanBeEditedAndStaleVersionsAreRejected(t *testing.T) {
	router, credentials, _, store, _ := newHTTPTest(t, true)
	create := url.Values{
		"name": {"Laundry liquid"}, "brand": {"Example"}, "line": {"Laundry"},
		"variant": {"Fresh"}, "quantity": {"2 x 750 ml"}, csrfFormField: {credentials.CSRF},
	}
	createRequest := authenticatedRequest(http.MethodPost, "http://example.test/products/new", strings.NewReader(create.Encode()), credentials)
	createRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	createRequest.Header.Set("Origin", "http://example.test")
	created := httptest.NewRecorder()
	router.ServeHTTP(created, createRequest)
	if created.Code != http.StatusSeeOther {
		t.Fatalf("create product = %d, body %s", created.Code, created.Body.String())
	}
	products, err := store.ListProducts(context.Background(), 10)
	if err != nil || len(products) != 1 {
		t.Fatalf("saved products = %+v, %v", products, err)
	}
	product := products[0]
	detailPage := httptest.NewRecorder()
	router.ServeHTTP(detailPage, authenticatedRequest(http.MethodGet, "http://example.test/products/"+product.ID, nil, credentials))
	if detailPage.Code != http.StatusOK || !strings.Contains(detailPage.Body.String(), "Latest verified offers") ||
		strings.Contains(detailPage.Body.String(), "Saved product variants") {
		t.Fatalf("product detail did not render its own offer/history view: %d %s", detailPage.Code, detailPage.Body.String())
	}
	editPage := httptest.NewRecorder()
	router.ServeHTTP(editPage, authenticatedRequest(http.MethodGet, "http://example.test/products/"+product.ID+"/edit", nil, credentials))
	if editPage.Code != http.StatusOK || !strings.Contains(editPage.Body.String(), `value="2 x 0.75 L"`) {
		t.Fatalf("edit page did not render exact pack quantity: %d %s", editPage.Code, editPage.Body.String())
	}
	if strings.Contains(editPage.Body.String(), "Saved product variants") {
		t.Fatal("product detail editor unexpectedly rendered the product list")
	}
	edit := url.Values{
		"name": {"Laundry liquid, updated"}, "brand": {"Example"}, "line": {"Laundry"},
		"variant": {"Fresh scent"}, "quantity": {"2 x 750 ml"}, "rationale": {"Corrected the variant after checking the label"},
		"expected_updated_at": {strconv.FormatInt(product.UpdatedAt, 10)}, csrfFormField: {credentials.CSRF},
	}
	editRequest := authenticatedRequest(http.MethodPost, "http://example.test/products/"+product.ID+"/edit", strings.NewReader(edit.Encode()), credentials)
	editRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	editRequest.Header.Set("Origin", "http://example.test")
	updated := httptest.NewRecorder()
	router.ServeHTTP(updated, editRequest)
	if updated.Code != http.StatusSeeOther || updated.Header().Get("Location") != "/products/"+product.ID {
		t.Fatalf("edit product = %d, location %q, body %s", updated.Code, updated.Header().Get("Location"), updated.Body.String())
	}
	current, err := store.ProductByID(context.Background(), product.ID)
	if err != nil || current.Name != "Laundry liquid, updated" || current.Variant != "Fresh scent" || current.Quantity != "3/4 L" || current.PackCount != 2 {
		t.Fatalf("updated canonical identity = %+v, %v", current, err)
	}
	edit.Set("name", "Stale edit")
	edit.Set("expected_updated_at", strconv.FormatInt(product.UpdatedAt, 10))
	staleRequest := authenticatedRequest(http.MethodPost, "http://example.test/products/"+product.ID+"/edit", strings.NewReader(edit.Encode()), credentials)
	staleRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	staleRequest.Header.Set("Origin", "http://example.test")
	stale := httptest.NewRecorder()
	router.ServeHTTP(stale, staleRequest)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "changed while you were editing") {
		t.Fatalf("stale edit = %d, body %s", stale.Code, stale.Body.String())
	}
}

func TestSourcesPageShowsAssessedButDisabledCandidates(t *testing.T) {
	router, credentials, _, _, _ := newHTTPTest(t, true)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, authenticatedRequest(http.MethodGet, "http://example.test/sources", nil, credentials))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Clicks") || !strings.Contains(response.Body.String(), "review_required") || !strings.Contains(response.Body.String(), "No retailer source is currently approved or enabled") {
		t.Fatalf("sources page did not show conservative assessment state: %d %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "Unavailable until approved") || strings.Contains(response.Body.String(), `action="/sources/source-clicks/enabled"`) {
		t.Fatalf("unapproved source displayed an enable action: %s", response.Body.String())
	}
	form := url.Values{"enabled": {"true"}, csrfFormField: {credentials.CSRF}}
	request := authenticatedRequest(http.MethodPost, "http://example.test/sources/source-clicks/enabled", strings.NewReader(form.Encode()), credentials)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.test")
	denied := httptest.NewRecorder()
	router.ServeHTTP(denied, request)
	if denied.Code != http.StatusConflict || !strings.Contains(denied.Body.String(), "not approved") {
		t.Fatalf("unapproved source enable action = %d %s", denied.Code, denied.Body.String())
	}
}

func TestHistoryCSVExportIsAuthenticatedPrivateAndFormulaSafe(t *testing.T) {
	router, credentials, _, _, _ := newHTTPTest(t, true)
	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "http://example.test/exports/history.csv", nil))
	if unauthenticated.Code != http.StatusSeeOther || unauthenticated.Header().Get("Location") != "/login" {
		t.Fatalf("unauthenticated export = %d, location %q", unauthenticated.Code, unauthenticated.Header().Get("Location"))
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, authenticatedRequest(http.MethodGet, "http://example.test/exports/history.csv", nil, credentials))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/csv; charset=utf-8" ||
		response.Header().Get("Cache-Control") != "private, no-store" || !strings.Contains(response.Body.String(), "observed_at,validation,offer_type") {
		t.Fatalf("history export response = %d %v %s", response.Code, response.Header(), response.Body.String())
	}
	for _, value := range []string{"=1+1", "+cmd", " @SUM(A1:A2)", "\t-HYPERLINK(\"https://bad.example\")"} {
		if escaped := safeSpreadsheetText(value); !strings.HasPrefix(escaped, "'") {
			t.Errorf("formula-like text %q was not escaped: %q", value, escaped)
		}
	}
	for _, value := range []string{"Laundry", "2-in-1", ""} {
		if escaped := safeSpreadsheetText(value); escaped != value {
			t.Errorf("ordinary text %q changed to %q", value, escaped)
		}
	}
}

func TestSafeListingURLRequiresExactHTTPSApprovedHost(t *testing.T) {
	for _, tc := range []struct {
		url  string
		host string
		want bool
	}{
		{"https://shop.example/product/123", "shop.example", true},
		{"https://sub.shop.example/product/123", "shop.example", false},
		{"http://shop.example/product/123", "shop.example", false},
		{"https://user:pass@shop.example/product/123", "shop.example", false},
		{"https://shop.example/product/123?token=secret", "shop.example", false},
		{"https://elsewhere.example/product/123", "shop.example", false},
	} {
		got := safeListingURL(tc.url, tc.host)
		if (got != "") != tc.want {
			t.Errorf("safeListingURL(%q,%q) = %q, want link=%v", tc.url, tc.host, got, tc.want)
		}
	}
}

func TestHealthEndpointsReflectReadiness(t *testing.T) {
	ready := false
	router, err := NewRouter(Options{Ready: func() bool { return ready }, Auth: testAuthService(t), RequestTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	for _, tc := range []struct {
		path string
		want int
	}{{"/health/live", http.StatusOK}, {"/health/ready", http.StatusServiceUnavailable}} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if recorder.Code != tc.want {
			t.Errorf("GET %s status = %d, want %d", tc.path, recorder.Code, tc.want)
		}
	}
	ready = true
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want 200", recorder.Code)
	}
}

func TestProtectedPagesAndLogoutRequireSessionAndCSRF(t *testing.T) {
	router, credentials, service, _, _ := newHTTPTest(t, true)
	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "http://example.test/", nil))
	if unauthenticated.Code != http.StatusSeeOther || unauthenticated.Header().Get("Location") != "/login" {
		t.Fatalf("unauthenticated page = %d, location %q", unauthenticated.Code, unauthenticated.Header().Get("Location"))
	}
	getLogout := httptest.NewRecorder()
	router.ServeHTTP(getLogout, authenticatedRequest(http.MethodGet, "http://example.test/logout", nil, credentials))
	if getLogout.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET logout status = %d, want 405", getLogout.Code)
	}
	if _, err := service.Authenticate(context.Background(), credentials.Token, time.Now().UTC()); err != nil {
		t.Fatalf("GET logout changed session: %v", err)
	}

	badRequest := httptest.NewRequest(http.MethodPost, "http://example.test/logout", strings.NewReader(""))
	badRequest.Header.Set("Origin", "http://example.test")
	badRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: credentials.Token})
	badRequest.AddCookie(&http.Cookie{Name: csrfCookieName, Value: credentials.CSRF})
	bad := httptest.NewRecorder()
	router.ServeHTTP(bad, badRequest)
	if bad.Code != http.StatusForbidden {
		t.Fatalf("logout without CSRF status = %d, want 403", bad.Code)
	}

	form := url.Values{csrfFormField: {credentials.CSRF}}
	logout := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodPost, "http://example.test/logout", strings.NewReader(form.Encode()), credentials)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.test")
	router.ServeHTTP(logout, request)
	if logout.Code != http.StatusSeeOther || logout.Header().Get("Location") != "/login" {
		t.Fatalf("valid logout = %d, location %q", logout.Code, logout.Header().Get("Location"))
	}
	if _, err := service.Authenticate(context.Background(), credentials.Token, time.Now().UTC()); err == nil {
		t.Fatal("logout left the session active")
	}
}

func TestSetupConsumesOneTimeTokenAndLoginUsesCSRF(t *testing.T) {
	router, _, service, _, token := newHTTPTest(t, false)
	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "http://example.test/setup", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `action="/setup"`) {
		t.Fatalf("GET setup = %d: %s", get.Code, get.Body.String())
	}
	var csrf string
	for _, cookie := range get.Result().Cookies() {
		if cookie.Name == loginCSRFName {
			csrf = cookie.Value
		}
	}
	if csrf == "" {
		t.Fatal("setup GET did not issue a CSRF cookie")
	}
	password := "Correct horse battery staple 2026!"
	form := url.Values{
		"setup_token":      {token},
		"username":         {"owner"},
		"password":         {password},
		"password_confirm": {password},
		csrfFormField:      {csrf},
	}
	post := httptest.NewRequest(http.MethodPost, "http://example.test/setup", strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("Origin", "http://example.test")
	post.AddCookie(&http.Cookie{Name: loginCSRFName, Value: csrf})
	created := httptest.NewRecorder()
	router.ServeHTTP(created, post)
	if created.Code != http.StatusSeeOther || created.Header().Get("Location") != "/" {
		t.Fatalf("POST setup = %d, location %q, body %s", created.Code, created.Header().Get("Location"), created.Body.String())
	}
	if !hasCookie(created.Result().Cookies(), sessionCookieName) || !hasCookie(created.Result().Cookies(), csrfCookieName) {
		t.Fatal("successful setup did not issue a session and CSRF cookie")
	}
	if matched, err := service.SetupTokenMatches(context.Background(), token); err != nil || matched {
		t.Fatalf("setup token replay state = %v, %v; want consumed", matched, err)
	}

	loginGet := httptest.NewRecorder()
	router.ServeHTTP(loginGet, httptest.NewRequest(http.MethodGet, "http://example.test/login", nil))
	var loginCSRF string
	for _, cookie := range loginGet.Result().Cookies() {
		if cookie.Name == loginCSRFName {
			loginCSRF = cookie.Value
		}
	}
	loginForm := url.Values{"username": {"owner"}, "password": {password}, csrfFormField: {loginCSRF}}
	loginPost := httptest.NewRequest(http.MethodPost, "http://example.test/login", strings.NewReader(loginForm.Encode()))
	loginPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginPost.Header.Set("Origin", "http://example.test")
	loginPost.AddCookie(&http.Cookie{Name: loginCSRFName, Value: loginCSRF})
	loggedIn := httptest.NewRecorder()
	router.ServeHTTP(loggedIn, loginPost)
	if loggedIn.Code != http.StatusSeeOther || loggedIn.Header().Get("Location") != "/" {
		t.Fatalf("POST login = %d, location %q", loggedIn.Code, loggedIn.Header().Get("Location"))
	}
}

func TestPasswordChangeRevokesAllSessions(t *testing.T) {
	router, credentials, service, _, _ := newHTTPTest(t, true)
	newPassword := "Another correct horse battery staple 2026!"
	form := url.Values{
		"current_password": {"Correct horse battery staple 2026!"},
		"new_password":     {newPassword},
		"password_confirm": {newPassword},
		csrfFormField:      {credentials.CSRF},
	}
	request := authenticatedRequest(http.MethodPost, "http://example.test/settings/password", strings.NewReader(form.Encode()), credentials)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.test")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/login" {
		t.Fatalf("password change = %d, location %q, body %s", recorder.Code, recorder.Header().Get("Location"), recorder.Body.String())
	}
	if _, err := service.Authenticate(context.Background(), credentials.Token, time.Now().UTC()); err == nil {
		t.Fatal("password change did not revoke the old session")
	}
	if _, err := service.Login(context.Background(), "owner", newPassword, "127.0.0.1", time.Now().UTC()); err != nil {
		t.Fatalf("login with changed password: %v", err)
	}
}

func TestSecureCookiesTrustOnlyConfiguredProxy(t *testing.T) {
	_, trusted, err := net.ParseCIDR("192.0.2.10/32")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "http://example.test/login", nil)
	request.RemoteAddr = "192.0.2.10:443"
	request.Header.Set("X-Forwarded-Proto", "https")
	if !isSecureRequest(request, []net.IPNet{*trusted}) {
		t.Fatal("trusted HTTPS reverse proxy was not recognized")
	}
	request.RemoteAddr = "198.51.100.23:443"
	if isSecureRequest(request, []net.IPNet{*trusted}) {
		t.Fatal("untrusted X-Forwarded-Proto header was accepted")
	}
}

func TestSecurityHeaders(t *testing.T) {
	router, credentials, _, _, _ := newHTTPTest(t, true)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authenticatedRequest(http.MethodGet, "http://example.test/", nil, credentials))
	for name, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	} {
		if got := recorder.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if got := recorder.Header().Get("X-Request-ID"); got == "" {
		t.Error("X-Request-ID is empty")
	}
}

func TestLocalFrontendAssetsAreEmbedded(t *testing.T) {
	router, _, _, _, _ := newHTTPTest(t, false)
	for _, path := range []string{"/static/js/htmx.min.js", "/static/css/app.css"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", path, recorder.Code)
		}
		if recorder.Body.Len() == 0 {
			t.Errorf("GET %s returned an empty asset", path)
		}
	}
}

func newHTTPTest(t *testing.T, createAdmin bool) (http.Handler, auth.Credentials, *auth.Service, *storage.Store, string) {
	t.Helper()
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "leanprice.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	hasher, err := auth.NewPasswordHasher()
	if err != nil {
		t.Fatal(err)
	}
	service, err := auth.NewService(store, hasher)
	if err != nil {
		t.Fatal(err)
	}
	token, err := auth.NewSetupToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SetSetupToken(context.Background(), token, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if createAdmin {
		if err := service.CreateInitialAdmin(context.Background(), token, "owner", "Correct horse battery staple 2026!", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}
	setupTokenPath := filepath.Join(t.TempDir(), "setup-token")
	if err := os.WriteFile(setupTokenPath, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	router, err := NewRouter(Options{Ready: func() bool { return true }, RequestTimeout: 2 * time.Second, Auth: service, Store: store, SetupTokenPath: setupTokenPath})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	var credentials auth.Credentials
	if createAdmin {
		credentials, err = service.Login(context.Background(), "owner", "Correct horse battery staple 2026!", "127.0.0.1", time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
	}
	return router, credentials, service, store, token
}

func testAuthService(t *testing.T) *auth.Service {
	t.Helper()
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "health.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	hasher, err := auth.NewPasswordHasher()
	if err != nil {
		t.Fatal(err)
	}
	service, err := auth.NewService(store, hasher)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func authenticatedRequest(method, target string, body io.Reader, credentials auth.Credentials) *http.Request {
	request := httptest.NewRequest(method, target, body)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: credentials.Token})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: credentials.CSRF})
	return request
}

func hasCookie(cookies []*http.Cookie, name string) bool {
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.Value != "" {
			return true
		}
	}
	return false
}
