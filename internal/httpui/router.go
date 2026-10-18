package httpui

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Onellan/leanPrice/internal/auth"
	"github.com/Onellan/leanPrice/internal/storage"
	"github.com/Onellan/leanPrice/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Options contains process-owned callbacks needed by the HTTP presentation.
type Options struct {
	Ready            func() bool
	RequestTimeout   time.Duration
	Auth             *auth.Service
	Store            *storage.Store
	SetupTokenPath   string
	TrustedProxies   []net.IPNet
	TestNotification func(context.Context, string) error
	MemoryStatus     func(context.Context) (ResourceStatus, error)
}

type ResourceStatus struct {
	CurrentBytes uint64
	SoftLimit    uint64
	ResumeAt     uint64
	Paused       bool
}

type page struct {
	Title                string
	Path                 string
	Heading              string
	Description          string
	EmptyTitle           string
	EmptyBody            string
	Navigation           []navigationItem
	CSRFToken            string
	Username             string
	Error                string
	Products             []storage.ProductSummary
	ProductOffers        []productOfferView
	PriceRange           *storage.ProductPriceRange
	PriceChart           []priceChartPoint
	Sources              []storage.SourceSummary
	Watches              []storage.WatchSummary
	Activity             []storage.ActivityItem
	Deals                []storage.DealSummary
	History              []storage.HistorySummary
	SelectedProduct      storage.ProductSummary
	ProductID            string
	ProductName          string
	FormError            string
	Quantity             string
	QuantityUnit         string
	PackCount            int
	Brand                string
	ProductLine          string
	Variant              string
	GTIN                 string
	TargetPrice          string
	DropPercentage       string
	ConditionOperator    string
	NotificationChannel  string
	WebhookConfigured    bool
	WatchID              string
	ExpectedRuleVersion  int
	WatchPreview         bool
	AllowMember          bool
	AllowMulti           bool
	Name                 string
	IsProductForm        bool
	IsWatchForm          bool
	IsWatchEdit          bool
	IsProductDetail      bool
	IsProductEdit        bool
	SearchQuery          string
	ExpectedUpdatedAt    int64
	Rationale            string
	FormAction           string
	SubmitLabel          string
	ShowNotificationTest bool
	Notice               string
	HasNextPage          bool
	NextPageURL          string
	ResourceStatus       *ResourceStatus
	ResourceUnavailable  bool
}

type navigationItem struct {
	Label string
	Path  string
}

type productOfferView struct {
	Retailer    string
	SafeURL     template.URL
	AmountMinor int64
	Currency    string
	Scale       int
	Stock       string
	ObservedAt  int64
	OfferKind   string
	Eligibility string
	MinimumQty  int
}

var navigation = []navigationItem{
	{Label: "Dashboard", Path: "/"},
	{Label: "Search", Path: "/search"},
	{Label: "Products", Path: "/products"},
	{Label: "Watches", Path: "/watches"},
	{Label: "Deals", Path: "/deals"},
	{Label: "Activity", Path: "/activity"},
	{Label: "Sources", Path: "/sources"},
	{Label: "Settings", Path: "/settings"},
}

var pages = []page{
	{Title: "Dashboard", Path: "/", Heading: "Your price watch", Description: "A clear view of verified offers, watched products and source health.", EmptyTitle: "Your watch list is ready to begin", EmptyBody: "Add a product from an assessed and enabled retailer to start keeping price history. No retailer sources are enabled yet."},
	{Title: "Search", Path: "/search", Heading: "Search your catalog", Description: "Find saved product variants while offline. Retailer search is unavailable until an eligible provider is approved and enabled."},
	{Title: "Products", Path: "/products", Heading: "Products", Description: "Keep exact product variants and retailer listings together.", EmptyTitle: "No products saved", EmptyBody: "Add the exact product variant you want to track. Retailer listings can be linked after a source is approved."},
	{Title: "Watches", Path: "/watches", Heading: "Watches", Description: "Set clear price conditions for products you want to track.", EmptyTitle: "No watches yet", EmptyBody: "Create a watch from a saved product. Alerts wait for fresh, exact, eligible offer evidence."},
	{Title: "Deals", Path: "/deals", Heading: "Verified offers", Description: "Only fresh, exact, public, in-stock offers from enabled sources appear here.", EmptyTitle: "No verified offers", EmptyBody: "No source-backed offers currently meet the verification rules. This does not mean that no retailer has a deal."},
	{Title: "Activity", Path: "/activity", Heading: "Activity", Description: "See what changed and why a check or alert needs attention.", EmptyTitle: "No activity yet", EmptyBody: "Checks, source status changes and alert episodes will appear here."},
	{Title: "Sources", Path: "/sources", Heading: "Retailer sources", Description: "Only assessed, permitted and enabled sources can be checked.", EmptyTitle: "No sources are configured", EmptyBody: "Source assessments and approvals are managed separately. No retailer source is currently enabled."},
	{Title: "Settings", Path: "/settings", Heading: "Settings", Description: "Local operation and safety controls for this leanPrice instance.", EmptyTitle: "Local settings", EmptyBody: "Manage your administrator credentials and use the documented local backup and restore commands."},
}

// NewRouter creates an SSR-first router. HTMX replaces only the main content;
// ordinary requests always receive a complete, navigable document.
func NewRouter(options Options) (http.Handler, error) {
	if options.Auth == nil {
		return nil, fmt.Errorf("authentication service is required")
	}
	if options.Ready == nil {
		options.Ready = func() bool { return false }
	}
	templateFS, err := fs.Sub(web.Assets, "templates")
	if err != nil {
		return nil, fmt.Errorf("open embedded templates: %w", err)
	}
	base, err := template.New("base").Funcs(template.FuncMap{
		"money": func(minor int64) string { return fmt.Sprintf("R%d.%02d", minor/100, minor%100) },
		"price": displayPrice,
		"dropPercent": func(basisPoints int64) string {
			return fmt.Sprintf("%d.%02d%%", basisPoints/100, basisPoints%100)
		},
		"timestamp": func(milliseconds int64) string {
			if milliseconds <= 0 {
				return "Never"
			}
			return time.UnixMilli(milliseconds).UTC().Format("2006-01-02 15:04 UTC")
		},
		"megabytes": func(bytes uint64) string {
			return fmt.Sprintf("%.1f", float64(bytes)/1_000_000)
		},
		"enabledSources": func(sources []storage.SourceSummary) int {
			count := 0
			for _, source := range sources {
				if source.Enabled {
					count++
				}
			}
			return count
		},
	}).ParseFS(templateFS, "layouts/base.html", "pages/page.html", "pages/auth.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	staticFS, err := fs.Sub(web.Assets, "static")
	if err != nil {
		return nil, fmt.Errorf("open embedded static assets: %w", err)
	}
	router := chi.NewRouter()
	if options.RequestTimeout <= 0 {
		options.RequestTimeout = 20 * time.Second
	}
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(options.RequestTimeout))
	router.Use(securityHeaders)
	router.Use(requestIDHeader)
	router.Get("/health/live", healthHandler(func() bool { return true }))
	router.Get("/health/ready", healthHandler(options.Ready))
	router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	authUI := authHandlers{service: options.Auth, templates: base, setupTokenPath: options.SetupTokenPath, trustedProxies: options.TrustedProxies}
	catalogUI := catalogHandlers{store: options.Store, templates: base, testNotification: options.TestNotification, memoryStatus: options.MemoryStatus}
	router.Get("/setup", authUI.setupGet)
	router.Post("/setup", authUI.setupPost)
	router.Get("/login", authUI.loginGet)
	router.Post("/login", authUI.loginPost)
	router.Group(func(protected chi.Router) {
		protected.Use(authUI.requireSession)
		protected.Post("/logout", authUI.logoutPost)
		protected.Post("/settings/password", authUI.changePasswordPost)
		protected.Post("/settings/notifications/test", catalogUI.testNotificationPost)
		protected.Post("/sources/{sourceID}/enabled", catalogUI.sourceEnabledPost)
		protected.Get("/products/new", catalogUI.productNewGet)
		protected.Post("/products/new", catalogUI.productNewPost)
		protected.Get("/products/{productID}/edit", catalogUI.productEditGet)
		protected.Post("/products/{productID}/edit", catalogUI.productEditPost)
		protected.Get("/products/{productID}", catalogUI.productDetailGet)
		protected.Get("/exports/history.csv", catalogUI.exportHistoryCSV)
		protected.Get("/watches/new", catalogUI.watchNewGet)
		protected.Post("/watches/new", catalogUI.watchNewPost)
		protected.Get("/watches/{watchID}/edit", catalogUI.watchEditGet)
		protected.Post("/watches/{watchID}/edit", catalogUI.watchEditPost)
		protected.Post("/watches/{watchID}/active", catalogUI.watchActivePost)
		for _, currentPage := range pages {
			currentPage := currentPage
			protected.Get(currentPage.Path, func(w http.ResponseWriter, r *http.Request) {
				data := currentPage
				data.Navigation = navigation
				data.CSRFToken = csrfCookieValue(r)
				data.ShowNotificationTest = options.TestNotification != nil
				if data.Path == "/search" {
					data.SearchQuery = strings.TrimSpace(r.URL.Query().Get("q"))
				}
				if err := validatePageCursor(r, data.Path); err != nil {
					http.Error(w, "Invalid page cursor", http.StatusBadRequest)
					return
				}
				if session, ok := r.Context().Value(sessionContextKey{}).(auth.SessionResult); ok {
					data.Username = session.User.Username
				}
				if err := catalogUI.populate(r, &data); err != nil {
					http.Error(w, "Page data is temporarily unavailable", http.StatusServiceUnavailable)
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("Cache-Control", "private, no-store")
				w.Header().Add("Vary", "HX-Request")
				if isHTMXRequest(r) {
					if err := base.ExecuteTemplate(w, "main", data); err != nil {
						http.Error(w, "Page could not be rendered", http.StatusInternalServerError)
					}
					return
				}
				if err := base.ExecuteTemplate(w, "base", data); err != nil {
					http.Error(w, "Page could not be rendered", http.StatusInternalServerError)
				}
			})
		}
	})
	router.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Page not found", http.StatusNotFound)
	})
	return router, nil
}

func isHTMXRequest(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("HX-Request")), "true")
}

func displayPrice(minor int64, currency string, scale int) string {
	if scale < 0 || scale > 3 {
		return fmt.Sprintf("%s %d minor units", currency, minor)
	}
	divisor := int64(1)
	for i := 0; i < scale; i++ {
		divisor *= 10
	}
	prefix := currency + " "
	if currency == "ZAR" {
		prefix = "R"
	}
	if scale == 0 {
		return prefix + fmt.Sprintf("%d", minor)
	}
	return prefix + fmt.Sprintf("%d.%0*d", minor/divisor, scale, minor%divisor)
}

func healthHandler(ready func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if !ready() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}
}

func requestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestID := middleware.GetReqID(r.Context()); requestID != "" {
			w.Header().Set("X-Request-ID", requestID)
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
