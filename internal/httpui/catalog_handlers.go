package httpui

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Onellan/leanPrice/internal/auth"
	"github.com/Onellan/leanPrice/internal/match"
	"github.com/Onellan/leanPrice/internal/pricing"
	"github.com/Onellan/leanPrice/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type catalogHandlers struct {
	store            *storage.Store
	templates        *template.Template
	testNotification func(context.Context, string) error
	memoryStatus     func(context.Context) (ResourceStatus, error)
}

type priceChartPoint struct {
	X               int
	Y               int
	AccessibleLabel string
}

func (h catalogHandlers) testNotificationPost(w http.ResponseWriter, r *http.Request) {
	if h.testNotification == nil {
		h.renderSettingsNotice(w, r, http.StatusServiceUnavailable, "", "Notification testing is not configured.")
		return
	}
	if err := h.testNotification(r.Context(), uuid.NewString()); err != nil {
		h.renderSettingsNotice(w, r, http.StatusServiceUnavailable, "", "The test notification could not be delivered. Check the configured destination and application logs.")
		return
	}
	h.renderSettingsNotice(w, r, http.StatusOK, "Test notification sent to the configured destination.", "")
}

func (h catalogHandlers) renderSettingsNotice(w http.ResponseWriter, r *http.Request, status int, notice, message string) {
	data := page{}
	for _, current := range pages {
		if current.Path == "/settings" {
			data = current
			break
		}
	}
	data.Navigation = navigation
	data.CSRFToken = csrfCookieValue(r)
	data.ShowNotificationTest = h.testNotification != nil
	if h.memoryStatus != nil {
		if status, err := h.memoryStatus(r.Context()); err != nil {
			data.ResourceUnavailable = true
		} else {
			data.ResourceStatus = &status
		}
	}
	data.Notice, data.Error = notice, message
	if session, ok := r.Context().Value(sessionContextKey{}).(auth.SessionResult); ok {
		data.Username = session.User.Username
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	if err := h.templates.ExecuteTemplate(w, "base", data); err != nil {
		return
	}
}

func (h catalogHandlers) populate(r *http.Request, data *page) error {
	if h.store == nil {
		return nil
	}
	ctx := r.Context()
	var err error
	switch data.Path {
	case "/":
		data.Products, err = h.store.ListProducts(ctx, 100)
		if err == nil {
			data.Watches, err = h.store.ListWatches(ctx, 100)
		}
		if err == nil {
			data.Sources, err = h.store.ListSources(ctx, 100)
		}
		if err == nil {
			data.Deals, err = h.store.ListVerifiedDeals(ctx, 100, time.Now().UTC())
		}
	case "/products":
		var more bool
		name, id := productPageCursor(r)
		data.Products, more, err = h.store.ListProductPage(ctx, "", name, id, storage.InteractivePageSize)
		data.HasNextPage = more
		if err == nil && more && len(data.Products) > 0 {
			data.NextPageURL = productNextPageURL(data.Path, "", data.Products[len(data.Products)-1])
		}
	case "/search":
		var more bool
		name, id := productPageCursor(r)
		data.Products, more, err = h.store.ListProductPage(ctx, data.SearchQuery, name, id, storage.InteractivePageSize)
		data.HasNextPage = more
		if err == nil && more && len(data.Products) > 0 {
			data.NextPageURL = productNextPageURL(data.Path, data.SearchQuery, data.Products[len(data.Products)-1])
		}
	case "/watches":
		data.Watches, err = h.store.ListWatches(ctx, 100)
		if err == nil {
			data.Products, err = h.store.ListProducts(ctx, 100)
		}
	case "/sources":
		data.Sources, err = h.store.ListSources(ctx, 100)
	case "/activity":
		var cursor *storage.ActivityCursor
		cursor, _, err = activityPageCursor(r)
		if err == nil {
			var more bool
			data.Activity, more, err = h.store.ListActivityPage(ctx, cursor, storage.InteractivePageSize)
			data.HasNextPage = more
			if err == nil && more && len(data.Activity) > 0 {
				data.NextPageURL = activityNextPageURL(data.Activity[len(data.Activity)-1])
			}
		}
	case "/deals":
		data.Deals, err = h.store.ListVerifiedDeals(ctx, 100, time.Now().UTC())
	case "/settings":
		if h.memoryStatus != nil {
			status, statusErr := h.memoryStatus(ctx)
			if statusErr != nil {
				data.ResourceUnavailable = true
			} else {
				data.ResourceStatus = &status
			}
		}
	}
	return err
}

func (h catalogHandlers) productDetailGet(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		http.Error(w, "Product storage is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	product, err := h.store.ProductByID(r.Context(), chi.URLParam(r, "productID"))
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Product details are temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	cursor, _, err := historyPageCursor(r)
	if err != nil {
		http.Error(w, "Invalid page cursor", http.StatusBadRequest)
		return
	}
	history, hasMoreHistory, err := h.store.ListProductHistoryPage(r.Context(), product.ID, cursor, storage.InteractivePageSize)
	if err != nil {
		http.Error(w, "Product history is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	now := time.Now().UTC()
	offers, err := h.store.ListProductOffers(r.Context(), product.ID, now, 100)
	if err != nil {
		http.Error(w, "Product offers are temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	views := make([]productOfferView, 0, len(offers))
	for _, offer := range offers {
		views = append(views, productOfferView{
			Retailer: offer.Retailer, SafeURL: safeListingURL(offer.ListingURL, offer.SourceHost),
			AmountMinor: offer.AmountMinor, Currency: offer.Currency, Scale: offer.Scale, Stock: offer.Stock,
			ObservedAt: offer.ObservedAt, OfferKind: offer.OfferKind, Eligibility: offer.Eligibility, MinimumQty: offer.MinimumQty,
		})
	}
	priceFrom := now.Add(-30 * 24 * time.Hour)
	priceRange, err := h.store.ProductObservedPriceRange(r.Context(), product.ID, priceFrom, now)
	if err != nil {
		http.Error(w, "Product price summary is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	priceSamples, err := h.store.ListProductPriceSamples(r.Context(), product.ID, priceFrom, now, 100)
	if err != nil {
		http.Error(w, "Product price history is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	data := page{Title: product.Name, Path: "/products", Heading: product.Name, Description: "Product identity and recorded observations. Historical values are not a promise of current availability.", Navigation: navigation, CSRFToken: csrfCookieValue(r), SelectedProduct: product, History: history, ProductOffers: views, PriceRange: priceRange, PriceChart: buildPriceChart(priceSamples), IsProductDetail: true, HasNextPage: hasMoreHistory}
	if hasMoreHistory && len(history) > 0 {
		data.NextPageURL = historyNextPageURL(product.ID, history[len(history)-1])
	}
	h.render(w, r, data)
}

func buildPriceChart(samples []storage.ProductPriceSample) []priceChartPoint {
	if len(samples) > 100 {
		samples = samples[len(samples)-100:]
	}
	if len(samples) == 0 {
		return nil
	}
	minAt, maxAt := samples[0].ObservedAt, samples[0].ObservedAt
	minPrice, maxPrice := samples[0].AmountMinor, samples[0].AmountMinor
	for _, sample := range samples[1:] {
		if sample.ObservedAt < minAt {
			minAt = sample.ObservedAt
		}
		if sample.ObservedAt > maxAt {
			maxAt = sample.ObservedAt
		}
		if sample.AmountMinor < minPrice {
			minPrice = sample.AmountMinor
		}
		if sample.AmountMinor > maxPrice {
			maxPrice = sample.AmountMinor
		}
	}
	points := make([]priceChartPoint, 0, len(samples))
	for _, sample := range samples {
		x, y := 160, 60
		if maxAt > minAt {
			x = 20 + int(float64(sample.ObservedAt-minAt)/float64(maxAt-minAt)*280)
		}
		if maxPrice > minPrice {
			y = 100 - int(float64(sample.AmountMinor-minPrice)/float64(maxPrice-minPrice)*80)
		}
		label := fmt.Sprintf("%s · R%d.%02d · %s", sample.Retailer, sample.AmountMinor/100, sample.AmountMinor%100,
			time.UnixMilli(sample.ObservedAt).UTC().Format("2006-01-02 15:04 UTC"))
		points = append(points, priceChartPoint{X: x, Y: y, AccessibleLabel: label})
	}
	return points
}

func safeListingURL(raw, expectedHost string) template.URL {
	if len(raw) == 0 || len(raw) > 2048 || strings.ContainsAny(raw, "\r\n\t") {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	expectedHost = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(expectedHost)), ".")
	if host == "" || host != expectedHost || parsed.Port() != "" && parsed.Port() != "443" {
		return ""
	}
	return template.URL(parsed.String())
}

func (h catalogHandlers) productEditGet(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		http.Error(w, "Product storage is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	product, err := h.store.ProductByID(r.Context(), chi.URLParam(r, "productID"))
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Product details are temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	quantity, err := formatEditableQuantity(product)
	if err != nil {
		http.Error(w, "This product quantity cannot be represented safely in the editor", http.StatusConflict)
		return
	}
	data := productEditPage(r, product, quantity)
	h.render(w, r, data)
}

func (h catalogHandlers) productEditPost(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		http.Error(w, "Product storage is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	productID := chi.URLParam(r, "productID")
	expectedUpdatedAt, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("expected_updated_at")), 10, 64)
	if err != nil || expectedUpdatedAt <= 0 {
		http.Error(w, "Invalid product edit version", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	quantityText := strings.TrimSpace(r.FormValue("quantity"))
	quantity, quantityErr := pricing.ParseQuantity(quantityText)
	brand, line, variant := strings.TrimSpace(r.FormValue("brand")), strings.TrimSpace(r.FormValue("line")), strings.TrimSpace(r.FormValue("variant"))
	if name == "" || len(name) > 240 || len(brand) > 120 || len(line) > 120 || len(variant) > 160 || quantityErr != nil {
		h.renderProductEditError(w, r, "Enter a product name and a valid quantity such as 750 ml, 1 L or 12 count.", expectedUpdatedAt)
		return
	}
	unit := map[pricing.Dimension]string{pricing.DimensionVolume: "L", pricing.DimensionMass: "kg", pricing.DimensionCount: "item"}[quantity.Dimension]
	gtin := strings.TrimSpace(r.FormValue("gtin"))
	if gtin != "" {
		normalized, valid := match.NormalizeGTIN(gtin)
		if !valid {
			h.renderProductEditError(w, r, "Enter a valid GTIN or leave the field blank.", expectedUpdatedAt)
			return
		}
		gtin = normalized
	}
	session, ok := r.Context().Value(sessionContextKey{}).(auth.SessionResult)
	if !ok || strings.TrimSpace(session.User.AdminID) == "" {
		http.Error(w, "Administrator session is unavailable", http.StatusUnauthorized)
		return
	}
	product := storage.ProductSummary{
		ID: productID, Name: name, Brand: brand, ProductLine: line, Variant: variant,
		Quantity:     strconv.FormatInt(quantity.Numerator, 10) + "/" + strconv.FormatInt(quantity.Denominator, 10),
		QuantityUnit: unit, PackCount: int(quantity.PackCount), GTIN: gtin,
	}
	rationale := strings.TrimSpace(r.FormValue("rationale"))
	if err := h.store.UpdateCanonicalProduct(r.Context(), session.User.AdminID, product, expectedUpdatedAt, rationale, time.Now().UTC()); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if errors.Is(err, storage.ErrConflict) {
			h.renderProductEditErrorStatus(w, r, "This product changed while you were editing it. Reopen the product and reapply your changes.", expectedUpdatedAt, http.StatusConflict)
			return
		}
		h.renderProductEditError(w, r, "The product identity could not be saved. Check the fields and edit rationale, then try again.", expectedUpdatedAt)
		return
	}
	http.Redirect(w, r, "/products/"+productID, http.StatusSeeOther)
}

func productEditPage(r *http.Request, product storage.ProductSummary, quantity string) page {
	return page{
		Title: "Edit product identity", Path: "/products", Heading: "Edit product identity",
		Description: "Identity changes are audited and exact retailer matches will require review again.",
		Navigation:  navigation, CSRFToken: csrfCookieValue(r), SelectedProduct: product, ProductID: product.ID,
		Name: product.Name, Brand: product.Brand, ProductLine: product.ProductLine, Variant: product.Variant,
		Quantity: quantity, GTIN: product.GTIN, IsProductForm: true, IsProductEdit: true,
		ExpectedUpdatedAt: product.UpdatedAt, FormAction: "/products/" + product.ID + "/edit",
	}
}

func (h catalogHandlers) renderProductEditError(w http.ResponseWriter, r *http.Request, message string, expectedUpdatedAt int64) {
	h.renderProductEditErrorStatus(w, r, message, expectedUpdatedAt, http.StatusUnprocessableEntity)
}

func (h catalogHandlers) renderProductEditErrorStatus(w http.ResponseWriter, r *http.Request, message string, expectedUpdatedAt int64, status int) {
	data := page{
		Title: "Edit product identity", Path: "/products", Heading: "Edit product identity",
		Description: "Identity changes are audited and exact retailer matches will require review again.",
		Navigation:  navigation, CSRFToken: csrfCookieValue(r), ProductID: chi.URLParam(r, "productID"),
		Name: r.FormValue("name"), Brand: r.FormValue("brand"), ProductLine: r.FormValue("line"), Variant: r.FormValue("variant"),
		Quantity: r.FormValue("quantity"), GTIN: r.FormValue("gtin"), Rationale: r.FormValue("rationale"),
		IsProductForm: true, IsProductEdit: true, ExpectedUpdatedAt: expectedUpdatedAt,
		FormAction: "/products/" + chi.URLParam(r, "productID") + "/edit", FormError: message,
	}
	if session, ok := r.Context().Value(sessionContextKey{}).(auth.SessionResult); ok {
		data.Username = session.User.Username
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	if err := h.templates.ExecuteTemplate(w, "base", data); err != nil {
		// Headers are committed above; the response remains safely failed with
		// the selected client error status if template execution fails.
		return
	}
}

func formatEditableQuantity(product storage.ProductSummary) (string, error) {
	quantityFields := strings.Fields(product.Quantity)
	if len(quantityFields) == 0 || len(quantityFields) > 2 || len(quantityFields) == 2 && quantityFields[1] != product.QuantityUnit {
		return "", pricing.ErrInvalidQuantity
	}
	parts := strings.Split(quantityFields[0], "/")
	if len(parts) != 2 {
		return "", pricing.ErrInvalidQuantity
	}
	numerator, numeratorOK := new(big.Int).SetString(parts[0], 10)
	denominator, denominatorOK := new(big.Int).SetString(parts[1], 10)
	if !numeratorOK || !denominatorOK || numerator.Sign() <= 0 || denominator.Sign() <= 0 || product.PackCount <= 0 {
		return "", pricing.ErrInvalidQuantity
	}
	unit, dimension := "", pricing.DimensionCount
	switch product.QuantityUnit {
	case "L":
		unit, dimension = "L", pricing.DimensionVolume
	case "kg":
		unit, dimension = "kg", pricing.DimensionMass
	case "item":
		unit = "count"
	default:
		return "", pricing.ErrInvalidQuantity
	}
	value := new(big.Rat).SetFrac(numerator, denominator).FloatString(3)
	value = strings.TrimRight(strings.TrimRight(value, "0"), ".")
	if value == "" {
		value = "0"
	}
	prefix := ""
	if product.PackCount > 1 {
		prefix = strconv.Itoa(product.PackCount) + " x "
	}
	text := prefix + value + " " + unit
	parsed, err := pricing.ParseQuantity(text)
	if err != nil || parsed.Numerator != productQuantityNumerator(product.Quantity) || parsed.Denominator != productQuantityDenominator(product.Quantity) ||
		parsed.Dimension != dimension || parsed.PackCount != int64(product.PackCount) {
		return "", pricing.ErrInvalidQuantity
	}
	return text, nil
}

func productQuantityNumerator(quantity string) int64 {
	fields := strings.Fields(quantity)
	if len(fields) == 0 {
		return 0
	}
	parts := strings.Split(fields[0], "/")
	if len(parts) != 2 {
		return 0
	}
	n, _ := strconv.ParseInt(parts[0], 10, 64)
	return n
}

func productQuantityDenominator(quantity string) int64 {
	fields := strings.Fields(quantity)
	if len(fields) == 0 {
		return 0
	}
	parts := strings.Split(fields[0], "/")
	if len(parts) != 2 {
		return 0
	}
	d, _ := strconv.ParseInt(parts[1], 10, 64)
	return d
}

func (h catalogHandlers) productNewGet(w http.ResponseWriter, r *http.Request) {
	data := page{Title: "Add product", Path: "/products", Heading: "Add a product", Description: "Enter one exact product variant. Quantity is normalized to litres, kilograms or items.", Navigation: navigation, CSRFToken: csrfCookieValue(r), QuantityUnit: "item", Quantity: "1", PackCount: 1, IsProductForm: true, FormAction: "/products/new"}
	h.render(w, r, data)
}

func (h catalogHandlers) productNewPost(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	quantityText := strings.TrimSpace(r.FormValue("quantity"))
	quantity, quantityErr := pricing.ParseQuantity(quantityText)
	if name == "" || len(name) > 240 || quantityErr != nil || len(r.FormValue("brand")) > 120 || len(r.FormValue("line")) > 120 || len(r.FormValue("variant")) > 160 {
		h.renderProductError(w, r, "Enter a product name and a valid quantity such as 750 ml, 1 L or 12 count.")
		return
	}
	unit := map[pricing.Dimension]string{pricing.DimensionVolume: "L", pricing.DimensionMass: "kg", pricing.DimensionCount: "item"}[quantity.Dimension]
	gtin := strings.TrimSpace(r.FormValue("gtin"))
	if gtin != "" {
		normalized, valid := match.NormalizeGTIN(gtin)
		if !valid {
			h.renderProductError(w, r, "Enter a valid GTIN or leave the field blank.")
			return
		}
		gtin = normalized
	}
	product := storage.ProductSummary{
		ID: uuid.NewString(), Name: name, Brand: strings.TrimSpace(r.FormValue("brand")),
		ProductLine: strings.TrimSpace(r.FormValue("line")), Variant: strings.TrimSpace(r.FormValue("variant")),
		Quantity:     strconv.FormatInt(quantity.Numerator, 10) + "/" + strconv.FormatInt(quantity.Denominator, 10),
		QuantityUnit: unit, PackCount: int(quantity.PackCount), GTIN: gtin,
	}
	if h.store == nil {
		http.Error(w, "Product storage is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := h.store.CreateCanonicalProduct(r.Context(), product, time.Now().UTC()); err != nil {
		h.renderProductError(w, r, "The product could not be saved. Check the identity fields and try again.")
		return
	}
	http.Redirect(w, r, "/products", http.StatusSeeOther)
}

func (h catalogHandlers) renderProductError(w http.ResponseWriter, r *http.Request, message string) {
	data := page{Title: "Add product", Path: "/products", Heading: "Add a product", Description: "Enter one exact product variant. Quantity is normalized to litres, kilograms or items.", Navigation: navigation, CSRFToken: csrfCookieValue(r), QuantityUnit: "item", Quantity: "1", PackCount: 1, IsProductForm: true, FormAction: "/products/new", FormError: message}
	data.Name, data.Brand, data.ProductLine, data.Variant, data.GTIN = r.FormValue("name"), r.FormValue("brand"), r.FormValue("line"), r.FormValue("variant"), r.FormValue("gtin")
	data.Quantity, data.QuantityUnit = r.FormValue("quantity"), r.FormValue("unit")
	h.render(w, r, data)
}

func (h catalogHandlers) watchNewGet(w http.ResponseWriter, r *http.Request) {
	data := page{Title: "New watch", Path: "/watches", Heading: "Create a price watch", Description: "A watch only alerts on fresh, exact and eligible observations.", Navigation: navigation, CSRFToken: csrfCookieValue(r), TargetPrice: "", ConditionOperator: "any", NotificationChannel: "in_app", WebhookConfigured: h.testNotification != nil, IsWatchForm: true, FormAction: "/watches/new", ProductID: strings.TrimSpace(r.URL.Query().Get("product"))}
	if h.store != nil {
		data.Products, _ = h.store.ListProducts(r.Context(), 100)
		h.addWatchPreview(r.Context(), &data)
	}
	h.render(w, r, data)
}

func (h catalogHandlers) watchNewPost(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		http.Error(w, "Watch storage is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	productID, rule, message := watchRuleFromRequest(r)
	if message != "" {
		h.renderWatchError(w, r, message)
		return
	}
	if rule.NotificationChannel == "webhook" && h.testNotification == nil {
		h.renderWatchError(w, r, "Choose an available notification destination.")
		return
	}
	if err := h.store.CreateWatch(r.Context(), uuid.NewString(), productID, "", true, rule, time.Now().UTC()); err != nil {
		h.renderWatchError(w, r, "The watch could not be saved. Choose an existing product and try again.")
		return
	}
	http.Redirect(w, r, "/watches", http.StatusSeeOther)
}

func watchRuleFromRequest(r *http.Request) (string, storage.WatchRule, string) {
	productID := strings.TrimSpace(r.FormValue("product_id"))
	targetText, dropText := strings.TrimSpace(r.FormValue("target_price")), strings.TrimSpace(r.FormValue("drop_percentage"))
	var targetMinor *int64
	if targetText != "" {
		target, err := pricing.ParseMoney(targetText, "ZAR")
		if err != nil {
			return productID, storage.WatchRule{}, "Enter a valid positive target price in rand or clear the field."
		}
		targetMinor = &target.Minor
	}
	var dropBPS *int
	if dropText != "" {
		percent, err := strconv.Atoi(dropText)
		if err != nil || percent < 1 || percent > 99 {
			return productID, storage.WatchRule{}, "Enter a whole-number price decrease from 1 to 99 percent or clear the field."
		}
		basisPoints := percent * 100
		dropBPS = &basisPoints
	}
	if targetMinor == nil && dropBPS == nil {
		return productID, storage.WatchRule{}, "Enter a target price, a relative price decrease, or both."
	}
	operator := "any"
	if targetMinor != nil && dropBPS != nil {
		operator = strings.TrimSpace(r.FormValue("condition_operator"))
		if operator != "any" && operator != "all" {
			return productID, storage.WatchRule{}, "Choose whether either or both conditions must be met."
		}
	}
	channel := strings.TrimSpace(r.FormValue("notification_channel"))
	if channel == "" {
		channel = "in_app"
	}
	if channel != "in_app" && channel != "webhook" {
		return productID, storage.WatchRule{}, "Choose an available notification destination."
	}
	if productID == "" {
		return productID, storage.WatchRule{}, "Choose a saved product."
	}
	rule := storage.WatchRule{TargetMinor: targetMinor, DropBasisPoints: dropBPS, ConditionOperator: operator,
		AllowMember: r.FormValue("allow_member") == "on", AllowMultibuy: r.FormValue("allow_multibuy") == "on",
		NotificationChannel: channel, RearmPolicyJSON: "{}"}
	return productID, rule, ""
}

func (h catalogHandlers) watchEditGet(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		http.Error(w, "Watch storage is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	watch, err := h.store.WatchByID(r.Context(), chi.URLParam(r, "watchID"))
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Watch is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	data := watchEditPage(r, watch)
	data.WebhookConfigured = h.testNotification != nil
	h.addWatchPreview(r.Context(), &data)
	h.render(w, r, data)
}

func (h catalogHandlers) watchEditPost(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		http.Error(w, "Watch storage is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	watchID := chi.URLParam(r, "watchID")
	watch, err := h.store.WatchByID(r.Context(), watchID)
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Watch is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	expectedVersion, parseErr := strconv.Atoi(strings.TrimSpace(r.FormValue("expected_rule_version")))
	if parseErr != nil || expectedVersion < 1 {
		h.renderWatchError(w, r, "This watch version is invalid. Reload the watch and try again.")
		return
	}
	productID, rule, message := watchRuleFromRequest(r)
	if message == "" && productID != watch.ProductID {
		message = "The product on a saved watch cannot be changed here."
	}
	if message == "" && rule.NotificationChannel == "webhook" && h.testNotification == nil && watch.NotificationChannel != "webhook" {
		message = "Configure a webhook before selecting it for this watch."
	}
	if message != "" {
		h.renderWatchError(w, r, message)
		return
	}
	if _, err := h.store.UpdateWatchRule(r.Context(), watchID, expectedVersion, rule, time.Now().UTC()); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if errors.Is(err, storage.ErrConflict) {
			http.Error(w, "This watch changed while you were editing it. Reload the current watch and apply your changes again.", http.StatusConflict)
			return
		}
		h.renderWatchError(w, r, "The watch could not be updated. Check the conditions and try again.")
		return
	}
	http.Redirect(w, r, "/watches", http.StatusSeeOther)
}

func watchEditPage(r *http.Request, watch storage.WatchSummary) page {
	target := ""
	if watch.TargetMinor.Valid {
		target = fmt.Sprintf("%d.%02d", watch.TargetMinor.Int64/100, watch.TargetMinor.Int64%100)
	}
	drop := ""
	if watch.DropBPS.Valid && watch.DropBPS.Int64%100 == 0 {
		drop = strconv.FormatInt(watch.DropBPS.Int64/100, 10)
	}
	return page{Title: "Edit watch", Path: "/watches", Heading: "Edit price watch", Description: "Changes create a new version; existing alert evidence keeps the rule version that produced it.",
		Navigation: navigation, CSRFToken: csrfCookieValue(r), ProductID: watch.ProductID, ProductName: watch.ProductName,
		WatchID: watch.ID, ExpectedRuleVersion: watch.RuleVersion, TargetPrice: target, DropPercentage: drop,
		ConditionOperator: watch.ConditionOperator, NotificationChannel: watch.NotificationChannel,
		WebhookConfigured: false, AllowMember: watch.AllowMember, AllowMulti: watch.AllowMulti, IsWatchForm: true, IsWatchEdit: true,
		FormAction: "/watches/" + url.PathEscape(watch.ID) + "/edit"}
}

func (h catalogHandlers) renderWatchError(w http.ResponseWriter, r *http.Request, message string) {
	data := page{Title: "New watch", Path: "/watches", Heading: "Create a price watch", Description: "A watch only alerts on fresh, exact and eligible observations.", Navigation: navigation, CSRFToken: csrfCookieValue(r), ProductID: r.FormValue("product_id"), TargetPrice: r.FormValue("target_price"), DropPercentage: r.FormValue("drop_percentage"), ConditionOperator: r.FormValue("condition_operator"), NotificationChannel: r.FormValue("notification_channel"), WebhookConfigured: h.testNotification != nil, IsWatchForm: true, FormAction: "/watches/new", FormError: message, AllowMember: r.FormValue("allow_member") == "on", AllowMulti: r.FormValue("allow_multibuy") == "on"}
	if h.store != nil {
		data.Products, _ = h.store.ListProducts(r.Context(), 100)
		if watchID := chi.URLParam(r, "watchID"); watchID != "" {
			if watch, err := h.store.WatchByID(r.Context(), watchID); err == nil {
				data.Title, data.Heading = "Edit watch", "Edit price watch"
				data.Description = "Changes create a new version; existing alert evidence keeps the rule version that produced it."
				data.IsWatchEdit, data.WatchID, data.ProductName = true, watch.ID, watch.ProductName
				data.ProductID, data.ExpectedRuleVersion = watch.ProductID, watch.RuleVersion
				data.FormAction = "/watches/" + url.PathEscape(watch.ID) + "/edit"
				if data.NotificationChannel == "" {
					data.NotificationChannel = watch.NotificationChannel
				}
			}
		}
		h.addWatchPreview(r.Context(), &data)
	}
	h.render(w, r, data)
}

func (h catalogHandlers) addWatchPreview(ctx context.Context, data *page) {
	if h.store == nil || data == nil || strings.TrimSpace(data.ProductID) == "" {
		return
	}
	product, err := h.store.ProductByID(ctx, data.ProductID)
	if err != nil {
		return
	}
	data.SelectedProduct, data.WatchPreview = product, true
	offers, err := h.store.ListProductOffers(ctx, product.ID, time.Now().UTC(), 50)
	if err != nil {
		return
	}
	data.ProductOffers = make([]productOfferView, 0, len(offers))
	for _, offer := range offers {
		data.ProductOffers = append(data.ProductOffers, productOfferView{
			Retailer: offer.Retailer, SafeURL: safeListingURL(offer.ListingURL, offer.SourceHost),
			AmountMinor: offer.AmountMinor, Currency: offer.Currency, Scale: offer.Scale,
			Stock: offer.Stock, ObservedAt: offer.ObservedAt, OfferKind: offer.OfferKind,
			Eligibility: offer.Eligibility, MinimumQty: offer.MinimumQty,
		})
	}
}

func (h catalogHandlers) watchActivePost(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		http.Error(w, "Watch storage is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	active, err := strconv.ParseBool(r.FormValue("active"))
	if err != nil {
		http.Error(w, "Invalid watch action", http.StatusBadRequest)
		return
	}
	if err := h.store.SetWatchActive(r.Context(), chi.URLParam(r, "watchID"), active); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Watch could not be updated", http.StatusServiceUnavailable)
		return
	}
	http.Redirect(w, r, "/watches", http.StatusSeeOther)
}

func (h catalogHandlers) sourceEnabledPost(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		http.Error(w, "Source storage is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	enabled, err := strconv.ParseBool(r.FormValue("enabled"))
	if err != nil {
		http.Error(w, "Invalid source action", http.StatusBadRequest)
		return
	}
	session, ok := r.Context().Value(sessionContextKey{}).(auth.SessionResult)
	if !ok || session.User.AdminID == "" {
		http.Error(w, "A valid administrator session is required", http.StatusUnauthorized)
		return
	}
	err = h.store.SetSourceEnabled(r.Context(), session.User.AdminID, chi.URLParam(r, "sourceID"), uuid.NewString(), uuid.NewString(), enabled, time.Now().UTC())
	switch {
	case errors.Is(err, storage.ErrNotFound):
		http.NotFound(w, r)
	case errors.Is(err, storage.ErrSourceNotApproved):
		http.Error(w, "This source is not approved and cannot be enabled", http.StatusConflict)
	case err != nil:
		http.Error(w, "Source state could not be updated", http.StatusServiceUnavailable)
	default:
		http.Redirect(w, r, "/sources", http.StatusSeeOther)
	}
}

func (h catalogHandlers) exportHistoryCSV(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		http.Error(w, "History export is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	items, err := h.store.ExportValidatedHistoryPage(r.Context(), 0, "", storage.MaxHistoryExportPage)
	if err != nil {
		http.Error(w, "History export is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=leanprice-history.csv")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Add("Trailer", "X-Export-Status")
	w.Header().Add("Trailer", "X-Export-Truncated")
	w.WriteHeader(http.StatusOK)
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"product", "brand", "product_line", "variant", "quantity", "pack_count", "retailer", "price", "currency", "stock", "observed_at", "validation", "offer_type", "minimum_quantity"}); err != nil {
		w.Header().Set("X-Export-Status", "failed")
		return
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		w.Header().Set("X-Export-Status", "failed")
		return
	}
	count := 0
	var afterAt int64
	afterID := ""
	for {
		for _, item := range items {
			if count >= storage.MaxHistoryExportRows {
				w.Header().Set("X-Export-Status", "complete")
				w.Header().Set("X-Export-Truncated", "true")
				return
			}
			row := []string{
				safeSpreadsheetText(item.ProductName), safeSpreadsheetText(item.Brand), safeSpreadsheetText(item.ProductLine),
				safeSpreadsheetText(item.Variant), safeSpreadsheetText(item.Quantity), strconv.Itoa(item.PackCount),
				safeSpreadsheetText(item.Retailer), formatCSVAmount(item.AmountMinor, item.Scale),
				safeSpreadsheetText(item.Currency), safeSpreadsheetText(item.Stock),
				time.UnixMilli(item.ObservedAt).UTC().Format(time.RFC3339), safeSpreadsheetText(item.Validation),
				safeSpreadsheetText(item.OfferKind), strconv.Itoa(item.MinimumQty),
			}
			if err := writer.Write(row); err != nil {
				w.Header().Set("X-Export-Status", "failed")
				return
			}
			count++
			afterAt, afterID = item.ObservedAt, item.OfferID
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			w.Header().Set("X-Export-Status", "failed")
			return
		}
		if len(items) < storage.MaxHistoryExportPage {
			w.Header().Set("X-Export-Status", "complete")
			w.Header().Set("X-Export-Truncated", "false")
			return
		}
		pageSize := storage.MaxHistoryExportPage
		if remaining := storage.MaxHistoryExportRows - count; remaining < pageSize {
			pageSize = remaining
		}
		if pageSize == 0 {
			probe, probeErr := h.store.ExportValidatedHistoryPage(r.Context(), afterAt, afterID, 1)
			if probeErr != nil {
				w.Header().Set("X-Export-Status", "failed")
				return
			}
			w.Header().Set("X-Export-Status", "complete")
			w.Header().Set("X-Export-Truncated", fmt.Sprintf("%t", len(probe) > 0))
			return
		}
		items, err = h.store.ExportValidatedHistoryPage(r.Context(), afterAt, afterID, pageSize)
		if err != nil {
			w.Header().Set("X-Export-Status", "failed")
			return
		}
	}
}

func safeSpreadsheetText(value string) string {
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			continue
		}
		if strings.ContainsRune("=+-@", r) {
			return "'" + value
		}
		return value
	}
	return value
}

func formatCSVAmount(minor int64, scale int) string {
	divisor := int64(1)
	for i := 0; i < scale; i++ {
		divisor *= 10
	}
	if scale == 0 {
		return strconv.FormatInt(minor, 10)
	}
	return fmt.Sprintf("%d.%0*d", minor/divisor, scale, minor%divisor)
}

func (h catalogHandlers) render(w http.ResponseWriter, r *http.Request, data page) {
	if session, ok := r.Context().Value(sessionContextKey{}).(auth.SessionResult); ok {
		data.Username = session.User.Username
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	if err := h.templates.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "Page could not be rendered", http.StatusInternalServerError)
	}
}
