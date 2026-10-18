// Package discovery defines bounded retailer-search providers. Search results
// are candidates only and never become prices or alerts without validation.
package discovery

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	MaxProviders  = 5
	MaxCandidates = 100
	MaxPageSize   = 50
)

var ErrDisabled = errors.New("discovery provider is not enabled")

type Candidate struct {
	ProviderID string
	ExternalID string
	Title      string
	URL        string
	GTIN       string
	Unverified bool
}

type Provider interface {
	ID() string
	Search(context.Context, string, int) ([]Candidate, error)
}

type Registry struct{ providers []Provider }

func (r *Registry) Register(provider Provider) error {
	if provider == nil || strings.TrimSpace(provider.ID()) == "" || len(provider.ID()) > 80 {
		return errors.New("discovery: provider needs a bounded stable ID")
	}
	if len(r.providers) >= MaxProviders {
		return errors.New("discovery: provider limit reached")
	}
	for _, existing := range r.providers {
		if existing.ID() == provider.ID() {
			return errors.New("discovery: duplicate provider ID")
		}
	}
	r.providers = append(r.providers, provider)
	return nil
}

func (r *Registry) Search(ctx context.Context, query string, pageSize int) ([]Candidate, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > 120 {
		return nil, errors.New("discovery: search query must contain 1–120 characters")
	}
	if pageSize < 1 || pageSize > MaxPageSize {
		return nil, errors.New("discovery: page size must be between 1 and 50")
	}
	results := make([]Candidate, 0, min(MaxCandidates, len(r.providers)*pageSize))
	seen := make(map[string]struct{}, MaxCandidates)
	for _, provider := range r.providers {
		if len(results) >= MaxCandidates {
			break
		}
		candidates, err := provider.Search(ctx, query, min(pageSize, MaxCandidates-len(results)))
		if err != nil {
			continue // A partial provider outage must not hide other results.
		}
		for _, candidate := range candidates {
			if len(results) >= MaxCandidates {
				break
			}
			candidate.ProviderID = provider.ID()
			candidate.Title = strings.TrimSpace(candidate.Title)
			if len(candidate.Title) > 240 || candidate.Title == "" || len(candidate.URL) > 2048 {
				continue
			}
			target, parseErr := url.Parse(candidate.URL)
			if parseErr != nil || target.Scheme != "https" || target.User != nil || target.Hostname() == "" {
				continue
			}
			key := strings.ToLower(target.Scheme + "://" + target.Host + target.EscapedPath())
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			candidate.Unverified = true
			results = append(results, candidate)
		}
	}
	return results, nil
}
