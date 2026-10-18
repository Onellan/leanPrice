// Package extract converts bounded source markup into provenance-bearing,
// deliberately untrusted product and offer facts.
package extract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/Onellan/leanPrice/internal/match"
	"github.com/Onellan/leanPrice/internal/pricing"
)

const (
	MaxMarkupBytes  = 2 << 20
	MaxJSONLDBytes  = 128 << 10
	MaxJSONLDBlocks = 32
	MaxDOMNodes     = 50000
	MaxJSONDepth    = 32
	ParserVersion   = "generic-jsonld-v1"
)

var (
	ErrMarkupTooLarge = errors.New("source markup exceeds parser limit")
	ErrTooManyNodes   = errors.New("source markup exceeds parser node limit")
	ErrInvalidInput   = errors.New("invalid extraction input")
)

type Status string

const (
	StatusParsed    Status = "parsed"
	StatusNoProduct Status = "not_found"
	StatusConflict  Status = "conflict"
)

type Origin struct {
	Kind string
	Path string
}

type Input struct {
	SourceURL  string
	ObservedAt time.Time
	Parser     string
	BodyDigest string
}

type ExtractedOffer struct {
	Offer         pricing.Offer
	Origin        Origin
	Valid         bool
	InvalidReason string
}

type Extraction struct {
	Status         Status
	SourceURL      string
	ObservedAt     time.Time
	ParserVersion  string
	EvidenceDigest string
	Title          string
	Brand          string
	Line           string
	Variant        string
	GTIN           string
	Quantity       *pricing.Quantity
	Stock          pricing.StockState
	Offers         []ExtractedOffer
	FieldOrigins   map[string]Origin
	Warnings       []string
}

type jsonProduct struct {
	data map[string]any
	path string
}

func Parse(markup []byte, input Input) (Extraction, error) {
	if len(markup) > MaxMarkupBytes {
		return Extraction{}, ErrMarkupTooLarge
	}
	if len(markup) == 0 || input.ObservedAt.IsZero() {
		return Extraction{}, ErrInvalidInput
	}
	sum := sha256.Sum256(markup)
	digest := hex.EncodeToString(sum[:])
	if input.BodyDigest != "" && !strings.EqualFold(input.BodyDigest, digest) {
		return Extraction{}, ErrInvalidInput
	}
	parser := input.Parser
	if parser == "" {
		parser = ParserVersion
	}
	result := Extraction{
		Status:         StatusNoProduct,
		SourceURL:      SanitizeURL(input.SourceURL),
		ObservedAt:     input.ObservedAt.UTC(),
		ParserVersion:  parser,
		EvidenceDigest: digest,
		Stock:          pricing.StockUnknown,
		FieldOrigins:   make(map[string]Origin),
	}

	doc, err := html.Parse(bytes.NewReader(markup))
	if err != nil {
		return Extraction{}, err
	}
	scripts := make([]string, 0, 4)
	products := make([]jsonProduct, 0, 2)
	var metaPrice string
	var metaCurrency string
	metaPriceConflict := false
	var visibleTitle string
	var visibleBrand string
	stack := []*html.Node{doc}
	nodes := 0
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		nodes++
		if nodes > MaxDOMNodes {
			return Extraction{}, ErrTooManyNodes
		}
		if node.Type == html.ElementNode {
			switch node.Data {
			case "script":
				if strings.EqualFold(attribute(node, "type"), "application/ld+json") {
					if len(scripts) >= MaxJSONLDBlocks {
						result.Warnings = append(result.Warnings, "jsonld_block_limit")
					} else {
						content := textContent(node)
						if len(content) > MaxJSONLDBytes {
							result.Warnings = append(result.Warnings, "jsonld_block_oversized")
						} else {
							scripts = append(scripts, content)
						}
					}
				}
			case "meta":
				prop := strings.ToLower(attribute(node, "property"))
				itemprop := strings.ToLower(attribute(node, "itemprop"))
				name := strings.ToLower(attribute(node, "name"))
				value := attribute(node, "content")
				switch {
				case prop == "product:price:amount" || itemprop == "price":
					if metaPrice == "" {
						metaPrice = value
					} else if strings.TrimSpace(metaPrice) != strings.TrimSpace(value) {
						metaPriceConflict = true
					}
				case prop == "product:price:currency" || itemprop == "pricecurrency":
					if metaCurrency == "" {
						metaCurrency = value
					} else if !strings.EqualFold(strings.TrimSpace(metaCurrency), strings.TrimSpace(value)) {
						metaPriceConflict = true
					}
				case prop == "og:title" || name == "title":
					if visibleTitle == "" {
						visibleTitle = strings.TrimSpace(value)
					}
				case itemprop == "brand":
					if visibleBrand == "" {
						visibleBrand = strings.TrimSpace(value)
					}
				}
			case "h1":
				if visibleTitle == "" {
					visibleTitle = strings.TrimSpace(textContent(node))
				}
			}
		}
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}

	for index, script := range scripts {
		decoder := json.NewDecoder(strings.NewReader(script))
		decoder.UseNumber()
		var root any
		if err := decoder.Decode(&root); err != nil {
			result.Warnings = append(result.Warnings, "invalid_jsonld")
			continue
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			result.Warnings = append(result.Warnings, "invalid_jsonld_trailing_data")
			continue
		}
		collected := make([]jsonProduct, 0, 2)
		if err := collectProducts(root, "jsonld["+itoa(index)+"]", 0, &collected); err != nil {
			result.Warnings = append(result.Warnings, "jsonld_depth_limit")
			continue
		}
		products = append(products, collected...)
	}
	if len(products) == 0 {
		if visibleTitle != "" {
			result.Title = visibleTitle
			result.FieldOrigins["title"] = Origin{Kind: "html_meta", Path: "title"}
		}
		return result, nil
	}
	if len(products) > 1 && conflictingProducts(products) {
		result.Status = StatusConflict
		result.Warnings = append(result.Warnings, "multiple_conflicting_products")
		return result, nil
	}
	if metaPriceConflict {
		result.Status = StatusConflict
		result.Warnings = append(result.Warnings, "conflicting_html_price_metadata")
		return result, nil
	}
	product := products[0]
	result.Status = StatusParsed
	setString(&result, "title", stringValue(product.data["name"]), product.path+".name")
	jsonTitle := result.Title
	jsonBrand := brandValue(product.data["brand"])
	if visibleTitle != "" && jsonTitle != "" && match.NormalizeText(visibleTitle) != match.NormalizeText(jsonTitle) {
		result.Status = StatusConflict
		result.Warnings = append(result.Warnings, "structured_title_conflict")
		return result, nil
	}
	if visibleBrand != "" && jsonBrand != "" && match.NormalizeText(visibleBrand) != match.NormalizeText(jsonBrand) {
		result.Status = StatusConflict
		result.Warnings = append(result.Warnings, "structured_brand_conflict")
		return result, nil
	}
	if result.Title == "" && visibleTitle != "" {
		result.Title = visibleTitle
		result.FieldOrigins["title"] = Origin{Kind: "html_meta", Path: "title"}
	}
	result.Brand = brandValue(product.data["brand"])
	if result.Brand != "" {
		result.FieldOrigins["brand"] = Origin{Kind: "json_ld", Path: product.path + ".brand"}
	} else if visibleBrand != "" {
		result.Brand = visibleBrand
		result.FieldOrigins["brand"] = Origin{Kind: "html_meta", Path: "itemprop=brand"}
	}
	var field string
	result.Line, field = firstStringKey(product.data, "productLine", "line")
	if result.Line != "" {
		result.FieldOrigins["line"] = Origin{Kind: "json_ld", Path: product.path + "." + field}
	}
	result.Variant, field = firstStringKey(product.data, "color", "scent", "flavor", "variant")
	if result.Variant != "" {
		result.FieldOrigins["variant"] = Origin{Kind: "json_ld", Path: product.path + "." + field}
	}
	result.GTIN, field = firstStringKey(product.data, "gtin", "gtin8", "gtin12", "gtin13", "gtin14")
	if normalized, valid := match.NormalizeGTIN(result.GTIN); valid {
		result.GTIN = normalized
		result.FieldOrigins["gtin"] = Origin{Kind: "json_ld", Path: product.path + "." + field}
	} else if result.GTIN != "" {
		result.Warnings = append(result.Warnings, "invalid_gtin")
		result.GTIN = ""
	}
	if size, key := firstStringKey(product.data, "size", "weight", "volume"); size != "" {
		if quantity, err := pricing.ParseQuantity(size); err == nil {
			result.Quantity = &quantity
			result.FieldOrigins["quantity"] = Origin{Kind: "json_ld", Path: product.path + "." + key}
		} else {
			result.Warnings = append(result.Warnings, "unparsed_quantity")
		}
	}
	result.Offers = extractOffers(product, result.ObservedAt)
	var observedStock pricing.StockState
	stockConsistent := true
	for _, offer := range result.Offers {
		if !offer.Valid {
			continue
		}
		if observedStock == "" {
			observedStock = offer.Offer.Stock
		} else if observedStock != offer.Offer.Stock {
			stockConsistent = false
		}
	}
	if stockConsistent && observedStock != "" {
		result.Stock = observedStock
	}
	if metaPrice != "" {
		metaOffer := parseMetaOffer(metaPrice, metaCurrency, result.ObservedAt)
		if metaOffer.Valid {
			if conflictWithOffers(metaOffer, result.Offers) {
				result.Status = StatusConflict
				result.Offers = nil
				result.Warnings = append(result.Warnings, "structured_price_conflict")
				return result, nil
			}
			hasValidOffer := false
			for _, offer := range result.Offers {
				hasValidOffer = hasValidOffer || offer.Valid
			}
			if !hasValidOffer {
				result.Offers = append(result.Offers, metaOffer)
			}
		}
	}
	if len(result.Offers) == 0 {
		result.Warnings = append(result.Warnings, "missing_valid_offer")
	}
	return result, nil
}

func collectProducts(value any, path string, depth int, products *[]jsonProduct) error {
	if depth > MaxJSONDepth || len(*products) > 128 {
		return ErrInvalidInput
	}
	switch typed := value.(type) {
	case []any:
		for index, item := range typed {
			if err := collectProducts(item, path+"["+itoa(index)+"]", depth+1, products); err != nil {
				return err
			}
		}
	case map[string]any:
		if isType(typed["@type"], "Product") {
			*products = append(*products, jsonProduct{data: typed, path: path})
		}
		for _, key := range []string{"@graph", "mainEntity", "itemListElement"} {
			if nested, ok := typed[key]; ok {
				if err := collectProducts(nested, path+"."+key, depth+1, products); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func isType(value any, wanted string) bool {
	switch typed := value.(type) {
	case string:
		return strings.EqualFold(typed, wanted) || strings.HasSuffix(strings.ToLower(typed), "/"+strings.ToLower(wanted))
	case []any:
		for _, item := range typed {
			if isType(item, wanted) {
				return true
			}
		}
	}
	return false
}

func conflictingProducts(products []jsonProduct) bool {
	first := products[0].data
	for _, product := range products[1:] {
		for _, key := range []string{"name", "productLine", "line", "gtin", "gtin8", "gtin12", "gtin13", "gtin14", "size", "color", "scent", "flavor", "variant"} {
			left, right := strings.TrimSpace(stringValue(first[key])), strings.TrimSpace(stringValue(product.data[key]))
			if left != "" && right != "" && !strings.EqualFold(left, right) {
				return true
			}
		}
		leftBrand, rightBrand := brandValue(first["brand"]), brandValue(product.data["brand"])
		if leftBrand != "" && rightBrand != "" && !strings.EqualFold(leftBrand, rightBrand) {
			return true
		}
		leftOffers, _ := json.Marshal(first["offers"])
		rightOffers, _ := json.Marshal(product.data["offers"])
		if len(leftOffers) > 0 && len(rightOffers) > 0 && !bytes.Equal(leftOffers, rightOffers) {
			return true
		}
	}
	return false
}

func extractOffers(product jsonProduct, observedAt time.Time) []ExtractedOffer {
	value := product.data["offers"]
	var offers []any
	switch typed := value.(type) {
	case []any:
		offers = typed
	case map[string]any:
		offers = []any{typed}
	}
	result := make([]ExtractedOffer, 0, len(offers))
	for index, value := range offers {
		data, ok := value.(map[string]any)
		if !ok {
			continue
		}
		price := stringValue(data["price"])
		pricePath := product.path + ".offers[" + itoa(index) + "].price"
		currency := strings.ToUpper(strings.TrimSpace(stringValue(data["priceCurrency"])))
		if price == "" {
			if specification, ok := data["priceSpecification"].(map[string]any); ok {
				price = stringValue(specification["price"])
				pricePath = product.path + ".offers[" + itoa(index) + "].priceSpecification.price"
				if currency == "" {
					currency = strings.ToUpper(strings.TrimSpace(stringValue(specification["priceCurrency"])))
				}
			}
		}
		offer := pricing.Offer{Type: pricing.OfferUnknown, Eligibility: pricing.EligibilityUnknown, MinimumQty: 1,
			TermsKnown: false, Stock: stockValue(stringValue(data["availability"])), ObservedAt: observedAt}
		extracted := ExtractedOffer{Offer: offer, Origin: Origin{Kind: "json_ld", Path: pricePath}}
		if price == "" || currency == "" {
			extracted.InvalidReason = "missing_price_or_currency"
			result = append(result, extracted)
			continue
		}
		money, err := pricing.ParseMoney(price, currency)
		if err != nil {
			extracted.InvalidReason = "invalid_price"
			result = append(result, extracted)
			continue
		}
		extracted.Offer.Money = money
		extracted.Valid = true
		result = append(result, extracted)
	}
	return result
}

func parseMetaOffer(price, currency string, observedAt time.Time) ExtractedOffer {
	offer := ExtractedOffer{Offer: pricing.Offer{Type: pricing.OfferUnknown, Eligibility: pricing.EligibilityUnknown,
		MinimumQty: 1, TermsKnown: false, Stock: pricing.StockUnknown, ObservedAt: observedAt},
		Origin: Origin{Kind: "html_meta", Path: "product:price:amount"}}
	if strings.TrimSpace(currency) == "" {
		offer.InvalidReason = "missing_currency"
		return offer
	}
	money, err := pricing.ParseMoney(price, currency)
	if err != nil {
		offer.InvalidReason = "invalid_price"
		return offer
	}
	offer.Offer.Money = money
	offer.Valid = true
	return offer
}

func conflictWithOffers(candidate ExtractedOffer, offers []ExtractedOffer) bool {
	for _, offer := range offers {
		if offer.Valid && (offer.Offer.Money.Currency != candidate.Offer.Money.Currency || offer.Offer.Money.Minor != candidate.Offer.Money.Minor) {
			return true
		}
	}
	return false
}

func stockValue(value string) pricing.StockState {
	value = strings.ToLower(value)
	if strings.HasSuffix(value, "instock") || value == "in_stock" {
		return pricing.StockInStock
	}
	if strings.HasSuffix(value, "outofstock") || value == "out_of_stock" {
		return pricing.StockOutOfStock
	}
	return pricing.StockUnknown
}

func brandValue(value any) string {
	if name := stringValue(value); name != "" {
		return strings.TrimSpace(name)
	}
	if object, ok := value.(map[string]any); ok {
		return strings.TrimSpace(stringValue(object["name"]))
	}
	return ""
}

func firstStringKey(data map[string]any, keys ...string) (string, string) {
	for _, key := range keys {
		if value := strings.TrimSpace(stringValue(data[key])); value != "" {
			return value, key
		}
	}
	return "", ""
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func setString(result *Extraction, field, value, path string) {
	if value == "" {
		return
	}
	switch field {
	case "title":
		result.Title = strings.TrimSpace(value)
	}
	result.FieldOrigins[field] = Origin{Kind: "json_ld", Path: path}
}

func attribute(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}

func textContent(node *html.Node) string {
	var builder strings.Builder
	stack := []*html.Node{node}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	return strings.TrimSpace(builder.String())
}

func SanitizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return ""
	}
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.RawFragment = ""
	return u.String()
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
