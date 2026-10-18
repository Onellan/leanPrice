package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Onellan/leanPrice/internal/match"
)

const (
	MaxCatalogRows         = 100
	InteractivePageSize    = 50
	MaxProductCursorIDSize = 80
)

type ProductSummary struct {
	ID             string
	Name           string
	Brand          string
	ProductLine    string
	Variant        string
	Quantity       string
	QuantityUnit   string
	PackCount      int
	GTIN           string
	ListingCount   int
	WatchCount     int
	LastObservedAt int64
	UpdatedAt      int64
	SortName       string
}

type SourceSummary struct {
	ID                string
	Retailer          string
	Key               string
	Host              string
	Assessment        string
	AssessmentRef     string
	AssessmentAt      int64
	Enabled           bool
	MinimumInterval   int64
	LastSuccessAt     int64
	LastFailure       string
	NextAllowedAt     int64
	ParserVersion     string
	ManualURL         bool
	Search            bool
	Watch             bool
	SpecialTerms      bool
	CapabilitiesKnown bool
}

type WatchSummary struct {
	ID                  string
	ProductID           string
	ProductName         string
	Active              bool
	RuleVersion         int
	TargetMinor         sql.NullInt64
	DropBPS             sql.NullInt64
	ConditionOperator   string
	AllowMember         bool
	AllowMulti          bool
	NotificationChannel string
}

type ActivityItem struct {
	At       int64
	EventKey string
	Kind     string
	Subject  string
	Status   string
	Detail   string
}

type DealSummary struct {
	ProductID   string
	ProductName string
	Retailer    string
	ListingURL  string
	AmountMinor int64
	Currency    string
	Scale       int
	Stock       string
	ObservedAt  int64
	OfferKind   string
}

type HistorySummary struct {
	ObservedAt    int64
	ObservationID string
	OfferID       string
	Retailer      string
	SourceURL     string
	FetchStatus   string
	Validation    string
	FailureCode   string
	Stock         string
	AmountMinor   sql.NullInt64
	Currency      string
	CurrencyScale int
	OfferKind     string
}

type ActivityCursor struct {
	At       int64
	EventKey string
}

type HistoryCursor struct {
	ObservedAt    int64
	ObservationID string
	OfferID       string
}

type ProductOfferSummary struct {
	ListingID   string
	Retailer    string
	SourceHost  string
	ListingURL  string
	AmountMinor int64
	Currency    string
	Scale       int
	Stock       string
	ObservedAt  int64
	OfferKind   string
	Eligibility string
	MinimumQty  int
}

// ProductPriceSample is one observed public, in-stock price point for the
// bounded historical chart. It contains no retailer listing URL.
type ProductPriceSample struct {
	ObservedAt  int64
	Retailer    string
	AmountMinor int64
	Currency    string
	Scale       int
}

type ProductPriceRange struct {
	MinimumMinor     int64
	MaximumMinor     int64
	ObservationCount int64
	RetailerCount    int64
	FirstObservedAt  int64
	LastObservedAt   int64
	Currency         string
	CurrencyScale    int
}

func (s *Store) CreateCanonicalProduct(ctx context.Context, product ProductSummary, now time.Time) error {
	if strings.TrimSpace(product.ID) == "" || strings.TrimSpace(product.Name) == "" || len(product.Name) > 240 ||
		product.PackCount < 1 || product.PackCount > 10000 {
		return errors.New("storage: product name, ID and bounded pack count are required")
	}
	if len(product.Brand) > 120 || len(product.ProductLine) > 120 || len(product.Variant) > 160 || len(product.GTIN) > 14 {
		return errors.New("storage: product identity field exceeds its limit")
	}
	parts := strings.Split(product.Quantity, "/")
	if len(parts) != 2 {
		return errors.New("storage: quantity must be canonical numerator/denominator")
	}
	var numerator, denominator int64
	if parsed, err := strconv.ParseInt(parts[0], 10, 64); err != nil || parsed < 1 {
		return errors.New("storage: quantity numerator must be positive")
	} else {
		numerator = parsed
	}
	if parsed, err := strconv.ParseInt(parts[1], 10, 64); err != nil || parsed < 1 {
		return errors.New("storage: quantity denominator must be positive")
	} else {
		denominator = parsed
	}
	if product.QuantityUnit == "" {
		return errors.New("storage: quantity unit is required")
	}
	dimension := ""
	switch product.QuantityUnit {
	case "L":
		dimension = "volume"
	case "kg":
		dimension = "mass"
	case "item":
		dimension = "count"
	default:
		return errors.New("storage: supported quantity units are L, kg and item")
	}
	if product.GTIN != "" {
		if normalized, valid := match.NormalizeGTIN(product.GTIN); !valid || normalized != product.GTIN {
			return errors.New("storage: GTIN must be a valid 8, 12, 13 or 14 digit code")
		}
	}
	nowMS := now.UTC().UnixMilli()
	if nowMS <= 0 {
		return errors.New("storage: product timestamp is invalid")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO canonical_products
		(id, display_name, brand, product_line, variant, quantity_numerator, quantity_denominator,
		 quantity_dimension, quantity_unit, pack_count, gtin, created_at, updated_at)
		VALUES (?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)`,
		product.ID, strings.TrimSpace(product.Name), strings.TrimSpace(product.Brand), strings.TrimSpace(product.ProductLine),
		strings.TrimSpace(product.Variant), numerator, denominator, dimension, product.QuantityUnit,
		product.PackCount, product.GTIN, nowMS, nowMS)
	if err != nil {
		return fmt.Errorf("storage: create canonical product: %w", err)
	}
	return nil
}

func (s *Store) ListProducts(ctx context.Context, limit int) ([]ProductSummary, error) {
	limit = boundedLimit(limit)
	rows, err := s.db.QueryContext(ctx, `SELECT p.id, p.display_name, COALESCE(p.brand,''), COALESCE(p.product_line,''), COALESCE(p.variant,''),
		p.quantity_numerator || '/' || p.quantity_denominator || ' ' || p.quantity_unit, p.pack_count,
		p.quantity_unit, COALESCE(p.gtin,''), (SELECT count(*) FROM listings l WHERE l.canonical_product_id=p.id),
		(SELECT count(*) FROM watches w WHERE w.canonical_product_id=p.id AND w.active=1),
		COALESCE((SELECT max(ob.observed_at) FROM listings l JOIN sources s ON s.id=l.source_id
		 JOIN observations ob ON ob.listing_id=l.id WHERE l.canonical_product_id=p.id AND l.match_state='exact'
		 AND s.enabled=1 AND s.assessment_state='approved'),0),p.updated_at
		FROM canonical_products p WHERE p.archived=0 ORDER BY lower(p.display_name), p.id LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: list products: %w", err)
	}
	defer rows.Close()
	products := make([]ProductSummary, 0, limit)
	for rows.Next() {
		var product ProductSummary
		if err := rows.Scan(&product.ID, &product.Name, &product.Brand, &product.ProductLine, &product.Variant, &product.Quantity,
			&product.PackCount, &product.QuantityUnit, &product.GTIN, &product.ListingCount, &product.WatchCount, &product.LastObservedAt, &product.UpdatedAt); err != nil {
			return nil, fmt.Errorf("storage: scan product: %w", err)
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

// ListProductPage returns a bounded keyset page ordered by normalized name and
// ID. The extra row signals whether another page exists without a count query.
func (s *Store) ListProductPage(ctx context.Context, search string, afterName, afterID string, limit int) ([]ProductSummary, bool, error) {
	search = strings.TrimSpace(search)
	if len(search) > 120 || len(afterName) > 240 || len(afterID) > MaxProductCursorIDSize || (afterName == "") != (afterID == "") {
		return nil, false, errors.New("storage: invalid product page cursor or search query")
	}
	if limit < 1 || limit > InteractivePageSize {
		return nil, false, errors.New("storage: product page size is outside the allowed range")
	}
	args := make([]any, 0, 8)
	where := `p.archived=0`
	if search != "" {
		escaped := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(search)
		where += ` AND (lower(p.display_name) LIKE lower(?) ESCAPE '\' OR lower(COALESCE(p.brand,'')) LIKE lower(?) ESCAPE '\' OR
			lower(COALESCE(p.product_line,'')) LIKE lower(?) ESCAPE '\' OR lower(COALESCE(p.variant,'')) LIKE lower(?) ESCAPE '\')`
		pattern := "%" + escaped + "%"
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if afterName != "" {
		where += ` AND (lower(p.display_name)>? OR (lower(p.display_name)=? AND p.id>?))`
		args = append(args, afterName, afterName, afterID)
	}
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, `SELECT p.id,p.display_name,COALESCE(p.brand,''),COALESCE(p.product_line,''),COALESCE(p.variant,''),
		p.quantity_numerator || '/' || p.quantity_denominator || ' ' || p.quantity_unit,p.pack_count,p.quantity_unit,
		COALESCE(p.gtin,''),(SELECT count(*) FROM listings l WHERE l.canonical_product_id=p.id),
		(SELECT count(*) FROM watches w WHERE w.canonical_product_id=p.id AND w.active=1),
		COALESCE((SELECT max(ob.observed_at) FROM listings l JOIN sources s ON s.id=l.source_id JOIN observations ob ON ob.listing_id=l.id
			WHERE l.canonical_product_id=p.id AND l.match_state='exact' AND s.assessment_state='approved' AND s.enabled=1),0),
		p.updated_at,lower(p.display_name)
		FROM canonical_products p WHERE `+where+` ORDER BY lower(p.display_name),p.id LIMIT ?`, args...)
	if err != nil {
		return nil, false, fmt.Errorf("storage: list product page: %w", err)
	}
	defer rows.Close()
	products := make([]ProductSummary, 0, limit+1)
	for rows.Next() {
		var product ProductSummary
		if err := rows.Scan(&product.ID, &product.Name, &product.Brand, &product.ProductLine, &product.Variant,
			&product.Quantity, &product.PackCount, &product.QuantityUnit, &product.GTIN, &product.ListingCount,
			&product.WatchCount, &product.LastObservedAt, &product.UpdatedAt, &product.SortName); err != nil {
			return nil, false, fmt.Errorf("storage: scan product page: %w", err)
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(products) > limit
	if hasMore {
		products = products[:limit]
	}
	return products, hasMore, nil
}

func (s *Store) SearchProducts(ctx context.Context, query string, limit int) ([]ProductSummary, error) {
	query = strings.TrimSpace(query)
	if len(query) > 120 {
		return nil, errors.New("storage: product search query exceeds 120 characters")
	}
	if query == "" {
		return s.ListProducts(ctx, limit)
	}
	limit = boundedLimit(limit)
	escaped := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(query)
	rows, err := s.db.QueryContext(ctx, `SELECT p.id,p.display_name,COALESCE(p.brand,''),COALESCE(p.product_line,''),COALESCE(p.variant,''),
		p.quantity_numerator || '/' || p.quantity_denominator || ' ' || p.quantity_unit,p.pack_count,p.quantity_unit,
		COALESCE(p.gtin,''),(SELECT count(*) FROM listings l WHERE l.canonical_product_id=p.id),
		(SELECT count(*) FROM watches w WHERE w.canonical_product_id=p.id AND w.active=1),
		COALESCE((SELECT max(ob.observed_at) FROM listings l JOIN sources s ON s.id=l.source_id JOIN observations ob ON ob.listing_id=l.id
			WHERE l.canonical_product_id=p.id AND l.match_state='exact' AND s.assessment_state='approved' AND s.enabled=1),0),p.updated_at
		FROM canonical_products p WHERE p.archived=0 AND
		(lower(p.display_name) LIKE lower(?) ESCAPE '\' OR lower(COALESCE(p.brand,'')) LIKE lower(?) ESCAPE '\' OR
		 lower(COALESCE(p.product_line,'')) LIKE lower(?) ESCAPE '\' OR lower(COALESCE(p.variant,'')) LIKE lower(?) ESCAPE '\')
		ORDER BY lower(p.display_name),p.id LIMIT ?`, "%"+escaped+"%", "%"+escaped+"%", "%"+escaped+"%", "%"+escaped+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("storage: search products: %w", err)
	}
	defer rows.Close()
	products := make([]ProductSummary, 0, limit)
	for rows.Next() {
		var product ProductSummary
		if err := rows.Scan(&product.ID, &product.Name, &product.Brand, &product.ProductLine, &product.Variant,
			&product.Quantity, &product.PackCount, &product.QuantityUnit, &product.GTIN, &product.ListingCount,
			&product.WatchCount, &product.LastObservedAt, &product.UpdatedAt); err != nil {
			return nil, fmt.Errorf("storage: scan product search: %w", err)
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

func (s *Store) ProductByID(ctx context.Context, id string) (ProductSummary, error) {
	var product ProductSummary
	err := s.db.QueryRowContext(ctx, `SELECT p.id,p.display_name,COALESCE(p.brand,''),COALESCE(p.product_line,''),COALESCE(p.variant,''),
		p.quantity_numerator || '/' || p.quantity_denominator || ' ' || p.quantity_unit,p.pack_count,p.quantity_unit,
		COALESCE(p.gtin,''),(SELECT count(*) FROM listings l WHERE l.canonical_product_id=p.id),
		(SELECT count(*) FROM watches w WHERE w.canonical_product_id=p.id AND w.active=1),
		COALESCE((SELECT max(ob.observed_at) FROM listings l JOIN sources s ON s.id=l.source_id JOIN observations ob ON ob.listing_id=l.id
			WHERE l.canonical_product_id=p.id AND l.match_state='exact' AND s.assessment_state='approved' AND s.enabled=1),0),p.updated_at
		FROM canonical_products p WHERE p.id=? AND p.archived=0`, id).Scan(&product.ID, &product.Name, &product.Brand, &product.ProductLine,
		&product.Variant, &product.Quantity, &product.PackCount, &product.QuantityUnit, &product.GTIN,
		&product.ListingCount, &product.WatchCount, &product.LastObservedAt, &product.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProductSummary{}, ErrNotFound
	}
	if err != nil {
		return ProductSummary{}, fmt.Errorf("storage: read product: %w", err)
	}
	return product, nil
}

func (s *Store) ListProductHistory(ctx context.Context, productID string, limit int) ([]HistorySummary, error) {
	limit = boundedLimit(limit)
	rows, err := s.db.QueryContext(ctx, `SELECT ob.observed_at,r.name,ob.source_url,ob.fetch_status,ob.validation_state,
		COALESCE(ob.failure_code,''),ob.observed_stock,o.amount_minor,COALESCE(o.currency,''),COALESCE(o.currency_scale,0),COALESCE(o.offer_kind,'')
		FROM observations ob JOIN listings l ON l.id=ob.listing_id JOIN sources s ON s.id=l.source_id JOIN retailers r ON r.id=s.retailer_id
		LEFT JOIN observed_offers o ON o.observation_id=ob.id
		WHERE l.canonical_product_id=? ORDER BY ob.observed_at DESC,ob.id DESC,o.id LIMIT ?`, productID, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: list product history: %w", err)
	}
	defer rows.Close()
	items := make([]HistorySummary, 0, limit)
	for rows.Next() {
		var item HistorySummary
		if err := rows.Scan(&item.ObservedAt, &item.Retailer, &item.SourceURL, &item.FetchStatus, &item.Validation,
			&item.FailureCode, &item.Stock, &item.AmountMinor, &item.Currency, &item.CurrencyScale, &item.OfferKind); err != nil {
			return nil, fmt.Errorf("storage: scan product history: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListProductHistoryPage uses observation and offer IDs as deterministic tie
// breakers so equal timestamps cannot repeat or disappear between pages.
func (s *Store) ListProductHistoryPage(ctx context.Context, productID string, after *HistoryCursor, limit int) ([]HistorySummary, bool, error) {
	if strings.TrimSpace(productID) == "" || limit < 1 || limit > InteractivePageSize {
		return nil, false, errors.New("storage: invalid product history page request")
	}
	where := `l.canonical_product_id=?`
	args := []any{productID}
	if after != nil {
		if after.ObservedAt <= 0 || after.ObservationID == "" || len(after.ObservationID) > 80 || len(after.OfferID) > 80 {
			return nil, false, errors.New("storage: invalid product history cursor")
		}
		where += ` AND (ob.observed_at<? OR (ob.observed_at=? AND ob.id<?) OR
			(ob.observed_at=? AND ob.id=? AND COALESCE(o.id,'')>?))`
		args = append(args, after.ObservedAt, after.ObservedAt, after.ObservationID, after.ObservedAt, after.ObservationID, after.OfferID)
	}
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, `SELECT ob.observed_at,ob.id,COALESCE(o.id,''),r.name,ob.source_url,ob.fetch_status,ob.validation_state,
		COALESCE(ob.failure_code,''),ob.observed_stock,o.amount_minor,COALESCE(o.currency,''),COALESCE(o.currency_scale,0),COALESCE(o.offer_kind,'')
		FROM observations ob JOIN listings l ON l.id=ob.listing_id JOIN sources s ON s.id=l.source_id JOIN retailers r ON r.id=s.retailer_id
		LEFT JOIN observed_offers o ON o.observation_id=ob.id WHERE `+where+`
		ORDER BY ob.observed_at DESC,ob.id DESC,COALESCE(o.id,'') ASC LIMIT ?`, args...)
	if err != nil {
		return nil, false, fmt.Errorf("storage: list product history page: %w", err)
	}
	defer rows.Close()
	items := make([]HistorySummary, 0, limit+1)
	for rows.Next() {
		var item HistorySummary
		if err := rows.Scan(&item.ObservedAt, &item.ObservationID, &item.OfferID, &item.Retailer, &item.SourceURL, &item.FetchStatus,
			&item.Validation, &item.FailureCode, &item.Stock, &item.AmountMinor, &item.Currency, &item.CurrencyScale, &item.OfferKind); err != nil {
			return nil, false, fmt.Errorf("storage: scan product history page: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

func (s *Store) ListSources(ctx context.Context, limit int) ([]SourceSummary, error) {
	limit = boundedLimit(limit)
	rows, err := s.db.QueryContext(ctx, `SELECT s.id, r.name, s.source_key, s.base_host, s.assessment_state,
		COALESCE(s.assessment_ref,''), COALESCE(s.assessment_at,0), s.enabled, s.min_interval_ms,s.capability_json,
		s.parser_version,s.next_allowed_at,
		COALESCE((SELECT max(ob.observed_at) FROM listings l JOIN observations ob ON ob.listing_id=l.id
			WHERE l.source_id=s.id AND ob.fetch_status='success'),0),
		COALESCE((SELECT COALESCE(ob.failure_code,ob.fetch_status) FROM listings l JOIN observations ob ON ob.listing_id=l.id
			WHERE l.source_id=s.id AND ob.fetch_status!='success' ORDER BY ob.observed_at DESC,ob.id DESC LIMIT 1),'')
		FROM sources s JOIN retailers r ON r.id=s.retailer_id ORDER BY lower(r.name),s.id LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: list sources: %w", err)
	}
	defer rows.Close()
	sources := make([]SourceSummary, 0, limit)
	for rows.Next() {
		var source SourceSummary
		var enabled int
		var capabilityJSON string
		if err := rows.Scan(&source.ID, &source.Retailer, &source.Key, &source.Host, &source.Assessment,
			&source.AssessmentRef, &source.AssessmentAt, &enabled, &source.MinimumInterval, &capabilityJSON,
			&source.ParserVersion, &source.NextAllowedAt, &source.LastSuccessAt, &source.LastFailure); err != nil {
			return nil, fmt.Errorf("storage: scan source: %w", err)
		}
		source.Enabled = enabled == 1
		var capabilities struct {
			ManualURL    bool `json:"manual_url"`
			Search       bool `json:"search"`
			Watch        bool `json:"watch"`
			SpecialTerms bool `json:"special_terms"`
		}
		if json.Unmarshal([]byte(capabilityJSON), &capabilities) == nil {
			source.CapabilitiesKnown = true
			source.ManualURL, source.Search, source.Watch, source.SpecialTerms = capabilities.ManualURL, capabilities.Search, capabilities.Watch, capabilities.SpecialTerms
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *Store) ListWatches(ctx context.Context, limit int) ([]WatchSummary, error) {
	limit = boundedLimit(limit)
	rows, err := s.db.QueryContext(ctx, `SELECT w.id,p.id,p.display_name,w.active,w.current_rule_version,
		rv.target_minor,rv.drop_basis_points,rv.condition_operator,rv.allow_member,rv.allow_multibuy,COALESCE(rv.notification_channel,'')
		FROM watches w JOIN canonical_products p ON p.id=w.canonical_product_id
		JOIN watch_rule_versions rv ON rv.watch_id=w.id AND rv.version=w.current_rule_version
		ORDER BY w.active DESC,w.created_at DESC,w.id LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: list watches: %w", err)
	}
	defer rows.Close()
	watches := make([]WatchSummary, 0, limit)
	for rows.Next() {
		var watch WatchSummary
		var active, member, multibuy int
		if err := rows.Scan(&watch.ID, &watch.ProductID, &watch.ProductName, &active, &watch.RuleVersion,
			&watch.TargetMinor, &watch.DropBPS, &watch.ConditionOperator, &member, &multibuy, &watch.NotificationChannel); err != nil {
			return nil, fmt.Errorf("storage: scan watch: %w", err)
		}
		watch.Active, watch.AllowMember, watch.AllowMulti = active == 1, member == 1, multibuy == 1
		watches = append(watches, watch)
	}
	return watches, rows.Err()
}

func (s *Store) WatchByID(ctx context.Context, watchID string) (WatchSummary, error) {
	var watch WatchSummary
	var active, member, multibuy int
	err := s.db.QueryRowContext(ctx, `SELECT w.id,p.id,p.display_name,w.active,w.current_rule_version,
		rv.target_minor,rv.drop_basis_points,rv.condition_operator,rv.allow_member,rv.allow_multibuy,COALESCE(rv.notification_channel,'')
		FROM watches w JOIN canonical_products p ON p.id=w.canonical_product_id
		JOIN watch_rule_versions rv ON rv.watch_id=w.id AND rv.version=w.current_rule_version WHERE w.id=?`, watchID).Scan(
		&watch.ID, &watch.ProductID, &watch.ProductName, &active, &watch.RuleVersion, &watch.TargetMinor, &watch.DropBPS,
		&watch.ConditionOperator, &member, &multibuy, &watch.NotificationChannel)
	if errors.Is(err, sql.ErrNoRows) {
		return WatchSummary{}, ErrNotFound
	}
	if err != nil {
		return WatchSummary{}, fmt.Errorf("storage: read watch: %w", err)
	}
	watch.Active, watch.AllowMember, watch.AllowMulti = active == 1, member == 1, multibuy == 1
	return watch, nil
}

func (s *Store) SetWatchActive(ctx context.Context, watchID string, active bool) error {
	value := 0
	if active {
		value = 1
	}
	result, err := s.db.ExecContext(ctx, "UPDATE watches SET active=? WHERE id=?", value, watchID)
	if err != nil {
		return fmt.Errorf("storage: set watch status: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListActivity(ctx context.Context, limit int) ([]ActivityItem, error) {
	limit = boundedLimit(limit)
	rows, err := s.db.QueryContext(ctx, `SELECT at,kind,subject,status,detail FROM (
		SELECT ob.observed_at AS at,'check' AS kind,p.display_name AS subject,ob.fetch_status AS status,
			COALESCE(ob.failure_code,'') AS detail FROM observations ob JOIN listings l ON l.id=ob.listing_id
			JOIN canonical_products p ON p.id=l.canonical_product_id
		UNION ALL
		SELECT e.created_at,'source',r.name,e.event_code,COALESCE(e.detail_redacted,'')
			FROM source_events e JOIN sources s ON s.id=e.source_id JOIN retailers r ON r.id=s.retailer_id
		UNION ALL
		SELECT a.created_at,'alert',p.display_name,CASE WHEN a.closed_at IS NULL THEN 'active' ELSE 'recovered' END,a.reason_code
			FROM alert_episodes a JOIN watches w ON w.id=a.watch_id JOIN canonical_products p ON p.id=w.canonical_product_id
	) ORDER BY at DESC,kind,subject LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: list activity: %w", err)
	}
	defer rows.Close()
	items := make([]ActivityItem, 0, limit)
	for rows.Next() {
		var item ActivityItem
		if err := rows.Scan(&item.At, &item.Kind, &item.Subject, &item.Status, &item.Detail); err != nil {
			return nil, fmt.Errorf("storage: scan activity: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListActivityPage returns bounded, stable keyset pages over the activity union.
func (s *Store) ListActivityPage(ctx context.Context, after *ActivityCursor, limit int) ([]ActivityItem, bool, error) {
	if limit < 1 || limit > InteractivePageSize {
		return nil, false, errors.New("storage: activity page size is outside the allowed range")
	}
	where := ""
	args := make([]any, 0, 4)
	if after != nil {
		if after.At <= 0 || after.EventKey == "" || len(after.EventKey) > 120 {
			return nil, false, errors.New("storage: invalid activity cursor")
		}
		where = ` WHERE at<? OR (at=? AND event_key>?)`
		args = append(args, after.At, after.At, after.EventKey)
	}
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, `SELECT at,event_key,kind,subject,status,detail FROM (
		SELECT ob.observed_at AS at,'check:'||ob.id AS event_key,'check' AS kind,p.display_name AS subject,ob.fetch_status AS status,
			COALESCE(ob.failure_code,'') AS detail FROM observations ob JOIN listings l ON l.id=ob.listing_id
			JOIN canonical_products p ON p.id=l.canonical_product_id
		UNION ALL
		SELECT e.created_at,'source:'||e.id,'source',r.name,e.event_code,COALESCE(e.detail_redacted,'')
			FROM source_events e JOIN sources s ON s.id=e.source_id JOIN retailers r ON r.id=s.retailer_id
		UNION ALL
		SELECT a.created_at,'alert:'||a.id,'alert',p.display_name,CASE WHEN a.closed_at IS NULL THEN 'active' ELSE 'recovered' END,a.reason_code
			FROM alert_episodes a JOIN watches w ON w.id=a.watch_id JOIN canonical_products p ON p.id=w.canonical_product_id
	)`+where+` ORDER BY at DESC,event_key ASC LIMIT ?`, args...)
	if err != nil {
		return nil, false, fmt.Errorf("storage: list activity page: %w", err)
	}
	defer rows.Close()
	items := make([]ActivityItem, 0, limit+1)
	for rows.Next() {
		var item ActivityItem
		if err := rows.Scan(&item.At, &item.EventKey, &item.Kind, &item.Subject, &item.Status, &item.Detail); err != nil {
			return nil, false, fmt.Errorf("storage: scan activity page: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

func (s *Store) ListVerifiedDeals(ctx context.Context, limit int, now time.Time) ([]DealSummary, error) {
	limit = boundedLimit(limit)
	rows, err := s.db.QueryContext(ctx, `SELECT p.id,p.display_name,r.name,l.url,o.amount_minor,o.currency,o.currency_scale,ob.observed_stock,
		ob.observed_at,o.offer_kind FROM observed_offers o JOIN observations ob ON ob.id=o.observation_id
		JOIN listings l ON l.id=ob.listing_id JOIN canonical_products p ON p.id=l.canonical_product_id
		JOIN sources s ON s.id=l.source_id JOIN retailers r ON r.id=s.retailer_id
		WHERE ob.fetch_status='success' AND ob.validation_state='valid' AND l.match_state='exact'
		AND s.assessment_state='approved' AND s.enabled=1 AND ob.observed_at<=? AND ob.observed_at>=?
		AND o.currency='ZAR' AND o.currency_scale=2
		AND o.eligibility_state='public' AND o.offer_kind IN ('standard','public_special')
		AND ob.observed_stock='in_stock' AND (o.valid_from IS NULL OR o.valid_from<=?) AND (o.valid_until IS NULL OR o.valid_until>?)
		AND ob.id=(SELECT ob2.id FROM observations ob2 WHERE ob2.listing_id=l.id ORDER BY ob2.observed_at DESC,ob2.id DESC LIMIT 1)
		ORDER BY o.amount_minor,ob.observed_at DESC LIMIT ?`, now.UTC().UnixMilli(), now.Add(-24*time.Hour).UTC().UnixMilli(), now.UTC().UnixMilli(), now.UTC().UnixMilli(), limit)
	if err != nil {
		return nil, fmt.Errorf("storage: list verified deals: %w", err)
	}
	defer rows.Close()
	deals := make([]DealSummary, 0, limit)
	for rows.Next() {
		var deal DealSummary
		if err := rows.Scan(&deal.ProductID, &deal.ProductName, &deal.Retailer, &deal.ListingURL,
			&deal.AmountMinor, &deal.Currency, &deal.Scale, &deal.Stock, &deal.ObservedAt, &deal.OfferKind); err != nil {
			return nil, fmt.Errorf("storage: scan verified deal: %w", err)
		}
		deals = append(deals, deal)
	}
	return deals, rows.Err()
}

func boundedLimit(limit int) int {
	if limit <= 0 || limit > MaxCatalogRows {
		return MaxCatalogRows
	}
	return limit
}
