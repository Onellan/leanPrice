package discovery

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	id         string
	candidates []Candidate
	err        error
	calls      int
}

func (p *fakeProvider) ID() string { return p.id }
func (p *fakeProvider) Search(_ context.Context, _ string, limit int) ([]Candidate, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	if len(p.candidates) > limit {
		return p.candidates[:limit], nil
	}
	return p.candidates, nil
}

func TestRegistryBoundsResultsAndMarksCandidatesUnverified(t *testing.T) {
	registry := &Registry{}
	first := &fakeProvider{id: "store-a", candidates: []Candidate{{Title: "One", URL: "https://shop.example/item?campaign=a"}, {Title: "Duplicate", URL: "https://shop.example/item?campaign=b"}, {Title: "Unsafe", URL: "http://shop.example/item"}}}
	second := &fakeProvider{id: "store-b", candidates: []Candidate{{Title: "Two", URL: "https://shop.example/other"}}}
	if err := registry.Register(first); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(second); err != nil {
		t.Fatal(err)
	}
	results, err := registry.Search(context.Background(), "soap", 10)
	if err != nil || len(results) != 2 || !results[0].Unverified || !results[1].Unverified {
		t.Fatalf("Search() = %+v, %v", results, err)
	}
}

func TestRegistryKeepsLocalSearchIndependentOfProviderFailure(t *testing.T) {
	registry := &Registry{}
	failing := &fakeProvider{id: "offline", err: errors.New("provider offline")}
	if err := registry.Register(failing); err != nil {
		t.Fatal(err)
	}
	results, err := registry.Search(context.Background(), "soap", 10)
	if err != nil || len(results) != 0 || failing.calls != 1 {
		t.Fatalf("Search() = %+v, %v; calls=%d", results, err, failing.calls)
	}
}
