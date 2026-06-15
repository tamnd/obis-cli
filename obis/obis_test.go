package obis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0 // no pacing in the test

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestSearchOccurrences(t *testing.T) {
	payload := wireSearchResp{
		Total: 2,
		Results: []wireOccurrence{
			{
				ID:             "occ-1",
				ScientificName: "Tursiops truncatus",
				Class:          "Mammalia",
				Family:         "Delphinidae",
				Genus:          "Tursiops",
				DecimalLat:     36.5,
				DecimalLon:     -6.2,
				Depth:          10.0,
				Year:           2020,
				Country:        "Spain",
				DatasetName:    "Mediterranean Survey",
				BasisOfRecord:  "HumanObservation",
			},
			{
				ID:             "occ-2",
				ScientificName: "Tursiops truncatus",
				Class:          "Mammalia",
				Family:         "Delphinidae",
				Year:           2019,
				Country:        "Italy",
				DatasetName:    "Adriatic Dataset",
			},
		},
		LastID: "occ-2",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/occurrence" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("scientificname"); got != "Tursiops truncatus" {
			t.Errorf("scientificname = %q, want %q", got, "Tursiops truncatus")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.HTTP.Transport = &prefixTransport{prefix: srv.URL, inner: http.DefaultTransport}

	occs, total, err := c.SearchOccurrences(context.Background(), "Tursiops truncatus", 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(occs) != 2 {
		t.Fatalf("len(occs) = %d, want 2", len(occs))
	}
	if occs[0].ID != "occ-1" {
		t.Errorf("occs[0].ID = %q, want occ-1", occs[0].ID)
	}
	if occs[0].ScientificName != "Tursiops truncatus" {
		t.Errorf("ScientificName = %q", occs[0].ScientificName)
	}
	if occs[0].Country != "Spain" {
		t.Errorf("Country = %q, want Spain", occs[0].Country)
	}
	if occs[0].Latitude != 36.5 {
		t.Errorf("Latitude = %v, want 36.5", occs[0].Latitude)
	}
}

func TestGetTaxon(t *testing.T) {
	payload := wireTaxonResp{
		Total: 1,
		Results: []wireTaxon{
			{
				TaxonID:        137094,
				ScientificName: "Tursiops truncatus",
				Rank:           "Species",
				Kingdom:        "Animalia",
				Phylum:         "Chordata",
				Class:          "Mammalia",
				Order:          "Artiodactyla",
				Family:         "Delphinidae",
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/taxon" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("scientificname"); got != "Tursiops truncatus" {
			t.Errorf("scientificname = %q, want %q", got, "Tursiops truncatus")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.HTTP.Transport = &prefixTransport{prefix: srv.URL, inner: http.DefaultTransport}

	taxa, err := c.GetTaxon(context.Background(), "Tursiops truncatus")
	if err != nil {
		t.Fatal(err)
	}
	if len(taxa) != 1 {
		t.Fatalf("len(taxa) = %d, want 1", len(taxa))
	}
	if taxa[0].ID != "137094" {
		t.Errorf("ID = %q, want 137094", taxa[0].ID)
	}
	if taxa[0].Kingdom != "Animalia" {
		t.Errorf("Kingdom = %q, want Animalia", taxa[0].Kingdom)
	}
	if taxa[0].Rank != "Species" {
		t.Errorf("Rank = %q, want Species", taxa[0].Rank)
	}
}

func TestListDatasets(t *testing.T) {
	payload := wireDatasetResp{
		Total: 6750,
		Results: []wireDataset{
			{ID: "ds-1", Title: "Mediterranean Survey", Url: "https://ipt.vliz.be/medsea"},
			{ID: "ds-2", Title: "Pacific Dataset", Url: "https://ipt.pac.org/pac"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/dataset" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.HTTP.Transport = &prefixTransport{prefix: srv.URL, inner: http.DefaultTransport}

	datasets, total, err := c.ListDatasets(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 6750 {
		t.Errorf("total = %d, want 6750", total)
	}
	if len(datasets) != 2 {
		t.Fatalf("len(datasets) = %d, want 2", len(datasets))
	}
	if datasets[0].ID != "ds-1" {
		t.Errorf("datasets[0].ID = %q, want ds-1", datasets[0].ID)
	}
	if datasets[0].URL != "https://ipt.vliz.be/medsea" {
		t.Errorf("datasets[0].URL = %q", datasets[0].URL)
	}
}

func TestGetStats(t *testing.T) {
	payload := wireStats{
		Records:  200287111,
		Species:  167568,
		Taxa:     197130,
		Datasets: 6750,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/statistics" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.HTTP.Transport = &prefixTransport{prefix: srv.URL, inner: http.DefaultTransport}

	stats, err := c.GetStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Records != 200287111 {
		t.Errorf("Records = %d, want 200287111", stats.Records)
	}
	if stats.Species != 167568 {
		t.Errorf("Species = %d, want 167568", stats.Species)
	}
	if stats.Datasets != 6750 {
		t.Errorf("Datasets = %d, want 6750", stats.Datasets)
	}
}

// prefixTransport rewrites request URLs so tests point to the httptest server
// instead of the real OBIS API, while preserving the path/query.
type prefixTransport struct {
	prefix string
	inner  http.RoundTripper
}

func (t *prefixTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.URL.Scheme = "http"
	r2.URL.Host = r.URL.Host
	// rewrite host to test server
	u := *r.URL
	u.Scheme = "http"
	// strip the scheme+host from prefix to get host only
	host := t.prefix
	if len(host) > 7 && host[:7] == "http://" {
		host = host[7:]
	}
	u.Host = host
	r2.URL = &u
	return t.inner.RoundTrip(r2)
}
