package httpui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Onellan/leanPrice/internal/auth"
)

const (
	sessionCookieName = "leanprice_session"
	csrfCookieName    = "leanprice_csrf"
	loginCSRFName     = "leanprice_login_csrf"
	csrfFormField     = "csrf_token"
	maxFormBytes      = 16 * 1024
)

type sessionContextKey struct{}

type authHandlers struct {
	service        *auth.Service
	templates      *template.Template
	setupTokenPath string
	trustedProxies []net.IPNet
}

type authPage struct {
	Title       string
	Heading     string
	Description string
	Mode        string
	CSRFToken   string
	Error       string
}

func (h authHandlers) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			h.expiredSession(w, r)
			return
		}
		session, err := h.service.Authenticate(r.Context(), cookie.Value, time.Now().UTC())
		if err != nil {
			if !errors.Is(err, auth.ErrNotFound) && !errors.Is(err, auth.ErrInvalidCredentials) {
				http.Error(w, "Authentication is temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			h.expiredSession(w, r)
			return
		}
		csrfToken := csrfCookieValue(r)
		if !auth.ValidCSRF(session, csrfToken, csrfToken) {
			h.expiredSession(w, r)
			return
		}
		if session.Refreshed {
			h.setSessionCookies(w, r, cookie.Value, csrfToken, time.UnixMilli(session.User.ExpiresAt))
		}
		if isMutation(r.Method) {
			if !sameOrigin(r, h.trustedProxies) {
				http.Error(w, "Request origin could not be verified", http.StatusForbidden)
				return
			}
			if err := readForm(w, r); err != nil {
				http.Error(w, "Invalid form submission", http.StatusBadRequest)
				return
			}
			cookieToken, _ := r.Cookie(csrfCookieName)
			submitted := r.FormValue(csrfFormField)
			if isHTMXRequest(r) && r.Header.Get("X-CSRF-Token") != "" {
				submitted = r.Header.Get("X-CSRF-Token")
			}
			csrfValue := ""
			if cookieToken != nil {
				csrfValue = cookieToken.Value
			}
			if !auth.ValidCSRF(session, csrfValue, submitted) {
				http.Error(w, "Request token is invalid or expired", http.StatusForbidden)
				return
			}
		}
		r = r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, session))
		next.ServeHTTP(w, r)
	})
}

func (h authHandlers) setupGet(w http.ResponseWriter, r *http.Request) {
	hasAdmin, err := h.service.HasAdmin(r.Context())
	if err != nil {
		http.Error(w, "Setup status is unavailable", http.StatusServiceUnavailable)
		return
	}
	if hasAdmin {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	token, err := newFormToken()
	if err != nil {
		http.Error(w, "Setup is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	setFormCookie(w, r, token, h.trustedProxies)
	h.renderAuth(w, http.StatusOK, authPage{Title: "Set up", Heading: "Create your administrator", Description: "Use the one-time token created for this local instance.", Mode: "setup", CSRFToken: token})
}

func (h authHandlers) setupPost(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r, h.trustedProxies) {
		http.Error(w, "Request origin could not be verified", http.StatusForbidden)
		return
	}
	if err := readForm(w, r); err != nil {
		http.Error(w, "Invalid form submission", http.StatusBadRequest)
		return
	}
	csrf := r.FormValue(csrfFormField)
	cookie, _ := r.Cookie(loginCSRFName)
	if cookie == nil || !constantEqual(cookie.Value, csrf) {
		http.Error(w, "Request token is invalid or expired", http.StatusForbidden)
		return
	}
	validSetupToken, err := h.service.SetupTokenMatches(r.Context(), r.FormValue("setup_token"))
	if err != nil {
		if errors.Is(err, auth.ErrRateLimited) {
			h.setupError(w, r, "Too many setup attempts. Wait before trying again.")
			return
		}
		http.Error(w, "Setup is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	if !validSetupToken {
		h.setupError(w, r, "The setup token is invalid, expired or already used.")
		return
	}
	password, confirmation := r.FormValue("password"), r.FormValue("password_confirm")
	if !constantEqual(password, confirmation) {
		h.setupError(w, r, "Passwords do not match.")
		return
	}
	if err := h.service.CreateInitialAdmin(r.Context(), r.FormValue("setup_token"), r.FormValue("username"), password, time.Now().UTC()); err != nil {
		switch {
		case strings.Contains(err.Error(), "password"), strings.Contains(err.Error(), "username"):
			h.setupError(w, r, err.Error())
		case errors.Is(err, auth.ErrInvalidSetupToken), errors.Is(err, auth.ErrSetupComplete):
			h.setupError(w, r, "The setup token is invalid, expired or already used.")
		case errors.Is(err, auth.ErrRateLimited):
			h.setupError(w, r, "Too many setup attempts. Wait before trying again.")
		default:
			http.Error(w, "Administrator setup could not be completed", http.StatusServiceUnavailable)
		}
		return
	}
	if h.setupTokenPath != "" {
		_ = os.Remove(h.setupTokenPath)
	}
	credentials, err := h.service.Login(r.Context(), r.FormValue("username"), password, remoteKey(r.RemoteAddr), time.Now().UTC())
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	h.setCredentials(w, r, credentials)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h authHandlers) loginGet(w http.ResponseWriter, r *http.Request) {
	hasAdmin, err := h.service.HasAdmin(r.Context())
	if err != nil {
		http.Error(w, "Login is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	if !hasAdmin {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	token, err := newFormToken()
	if err != nil {
		http.Error(w, "Login is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	setLoginFormCookie(w, r, token, h.trustedProxies)
	h.renderAuth(w, http.StatusOK, authPage{Title: "Sign in", Heading: "Sign in", Description: "Sign in to your private leanPrice instance.", Mode: "login", CSRFToken: token})
}

func (h authHandlers) loginPost(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r, h.trustedProxies) {
		http.Error(w, "Request origin could not be verified", http.StatusForbidden)
		return
	}
	if err := readForm(w, r); err != nil {
		http.Error(w, "Invalid form submission", http.StatusBadRequest)
		return
	}
	cookie, _ := r.Cookie(loginCSRFName)
	if cookie == nil || !constantEqual(cookie.Value, r.FormValue(csrfFormField)) {
		http.Error(w, "Request token is invalid or expired", http.StatusForbidden)
		return
	}
	credentials, err := h.service.Login(r.Context(), r.FormValue("username"), r.FormValue("password"), remoteKey(r.RemoteAddr), time.Now().UTC())
	if err != nil {
		if errors.Is(err, auth.ErrRateLimited) {
			h.renderAuth(w, http.StatusTooManyRequests, authPage{Title: "Sign in", Heading: "Sign in", Description: "Sign in to your private leanPrice instance.", Mode: "login", CSRFToken: cookie.Value, Error: "Too many sign-in attempts. Wait before trying again."})
			return
		}
		if errors.Is(err, auth.ErrInvalidCredentials) {
			h.renderAuth(w, http.StatusUnauthorized, authPage{Title: "Sign in", Heading: "Sign in", Description: "Sign in to your private leanPrice instance.", Mode: "login", CSRFToken: cookie.Value, Error: "The name or password was not accepted."})
			return
		}
		http.Error(w, "Login is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	h.setCredentials(w, r, credentials)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h authHandlers) logoutPost(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if err := h.service.Logout(r.Context(), cookie.Value); err != nil {
			http.Error(w, "Sign out could not be completed", http.StatusServiceUnavailable)
			return
		}
	}
	clearCookie(w, sessionCookieName, true, isSecureRequest(r, h.trustedProxies))
	clearCookie(w, csrfCookieName, false, isSecureRequest(r, h.trustedProxies))
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h authHandlers) changePasswordPost(w http.ResponseWriter, r *http.Request) {
	session, ok := r.Context().Value(sessionContextKey{}).(auth.SessionResult)
	if !ok {
		http.Error(w, "Sign in required", http.StatusUnauthorized)
		return
	}
	newPassword, confirmation := r.FormValue("new_password"), r.FormValue("password_confirm")
	if !constantEqual(newPassword, confirmation) {
		h.renderSettingsError(w, r, session, "New passwords do not match.")
		return
	}
	err := h.service.ChangePassword(r.Context(), session.User.AdminID, session.User.Username, r.FormValue("current_password"), newPassword, time.Now().UTC())
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			h.renderSettingsError(w, r, session, "The current password was not accepted.")
			return
		}
		if strings.Contains(err.Error(), "password") {
			h.renderSettingsError(w, r, session, err.Error())
			return
		}
		http.Error(w, "Password could not be changed", http.StatusServiceUnavailable)
		return
	}
	clearCookie(w, sessionCookieName, true, isSecureRequest(r, h.trustedProxies))
	clearCookie(w, csrfCookieName, false, isSecureRequest(r, h.trustedProxies))
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h authHandlers) renderSettingsError(w http.ResponseWriter, r *http.Request, session auth.SessionResult, message string) {
	data := page{}
	for _, current := range pages {
		if current.Path == "/settings" {
			data = current
			break
		}
	}
	data.Navigation = navigation
	data.CSRFToken = csrfCookieValue(r)
	data.Username = session.User.Username
	data.Error = message
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusUnprocessableEntity)
	if err := h.templates.ExecuteTemplate(w, "base", data); err != nil {
		return
	}
}

func (h authHandlers) setupError(w http.ResponseWriter, r *http.Request, message string) {
	token, err := newFormToken()
	if err != nil {
		http.Error(w, "Setup is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	setLoginFormCookie(w, r, token, h.trustedProxies)
	h.renderAuth(w, http.StatusUnprocessableEntity, authPage{Title: "Set up", Heading: "Create your administrator", Description: "Use the one-time token created for this local instance.", Mode: "setup", CSRFToken: token, Error: message})
}

func (h authHandlers) renderAuth(w http.ResponseWriter, status int, data authPage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := h.templates.ExecuteTemplate(w, "auth", data); err != nil {
		return
	}
}

func (h authHandlers) setCredentials(w http.ResponseWriter, r *http.Request, credentials auth.Credentials) {
	expires := time.UnixMilli(credentials.User.ExpiresAt)
	h.setSessionCookies(w, r, credentials.Token, credentials.CSRF, expires)
	clearCookie(w, loginCSRFName, false, isSecureRequest(r, h.trustedProxies))
}

func (h authHandlers) setSessionCookies(w http.ResponseWriter, r *http.Request, sessionToken, csrfToken string, expires time.Time) {
	secure := isSecureRequest(r, h.trustedProxies)
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 1 {
		maxAge = int(auth.SessionLifetime.Seconds())
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: sessionToken, Path: "/", Expires: expires, MaxAge: maxAge, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: csrfToken, Path: "/", Expires: expires, MaxAge: maxAge, Secure: secure, SameSite: http.SameSiteStrictMode})
}

func (h authHandlers) expiredSession(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, sessionCookieName, true, isSecureRequest(r, h.trustedProxies))
	clearCookie(w, csrfCookieName, false, isSecureRequest(r, h.trustedProxies))
	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/login")
		http.Error(w, "Sign in required", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func readForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	return r.ParseForm()
}

func setFormCookie(w http.ResponseWriter, r *http.Request, token string, proxies []net.IPNet) {
	setLoginFormCookie(w, r, token, proxies)
}

func setLoginFormCookie(w http.ResponseWriter, r *http.Request, token string, proxies []net.IPNet) {
	http.SetCookie(w, &http.Cookie{Name: loginCSRFName, Value: token, Path: "/", MaxAge: 600, HttpOnly: false, Secure: isSecureRequest(r, proxies), SameSite: http.SameSiteStrictMode})
}

func clearCookie(w http.ResponseWriter, name string, httpOnly, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: httpOnly, Secure: secure, SameSite: http.SameSiteStrictMode})
}

func csrfCookieValue(r *http.Request) string {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func newFormToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func constantEqual(left, right string) bool {
	if len(left) == 0 || len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func sameOrigin(r *http.Request, proxies []net.IPNet) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if !strings.EqualFold(parsed.Host, r.Host) {
		return false
	}
	if isSecureRequest(r, proxies) {
		return strings.EqualFold(parsed.Scheme, "https")
	}
	return strings.EqualFold(parsed.Scheme, "http")
}

func isSecureRequest(r *http.Request, proxies []net.IPNet) bool {
	if r.TLS != nil {
		return true
	}
	if !isTrustedProxy(r.RemoteAddr, proxies) {
		return false
	}
	forwardedProto := strings.TrimSpace(strings.SplitN(r.Header.Get("X-Forwarded-Proto"), ",", 2)[0])
	return strings.EqualFold(forwardedProto, "https")
}

func isTrustedProxy(remoteAddr string, proxies []net.IPNet) bool {
	if len(proxies) == 0 {
		return false
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = strings.Trim(remoteAddr, "[]")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, subnet := range proxies {
		if subnet.Contains(ip) {
			return true
		}
	}
	return false
}

func remoteKey(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func isMutation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
