// Package obis is the library behind the obis command line:
// the HTTP client, request shaping, and the typed data models for the
// Ocean Biodiversity Information System (OBIS) API at api.obis.org/v3.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package obis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// DefaultUserAgent identifies the client to OBIS.
const DefaultUserAgent = "obis/dev (+https://github.com/tamnd/obis-cli)"

// Host is the API host this client talks to.
const Host = "api.obis.org"

// BaseURL is the API root every request is built from.
const BaseURL = "https://" + Host + "/v3"

// Client talks to the OBIS API over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults: a 30s timeout, a 300ms
// minimum gap between requests, and three retries on transient errors.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		UserAgent: DefaultUserAgent,
		Rate:      300 * time.Millisecond,
		Retries:   3,
	}
}

// Get fetches url and returns the response body. It paces and retries according
// to the client's settings. The caller owns nothing extra; the body is read
// fully and closed here.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// --- wire types ---

type wireOccurrence struct {
	ID             string  `json:"id"`
	ScientificName string  `json:"scientificName"`
	Class          string  `json:"class"`
	Order          string  `json:"order"`
	Family         string  `json:"family"`
	Genus          string  `json:"genus"`
	DecimalLat     float64 `json:"decimalLatitude"`
	DecimalLon     float64 `json:"decimalLongitude"`
	Depth          float64 `json:"depth"`
	Year           int     `json:"year"`
	Month          int     `json:"month"`
	Day            int     `json:"day"`
	Country        string  `json:"country"`
	DatasetName    string  `json:"datasetName"`
	DatasetID      string  `json:"datasetID"`
	BasisOfRecord  string  `json:"basisOfRecord"`
}

type wireSearchResp struct {
	Total   int              `json:"total"`
	Results []wireOccurrence `json:"results"`
	LastID  string           `json:"lastID"`
}

type wireTaxon struct {
	TaxonID        int    `json:"taxonID"`
	ScientificName string `json:"scientificName"`
	Rank           string `json:"taxonRank"`
	Kingdom        string `json:"kingdom"`
	Phylum         string `json:"phylum"`
	Class          string `json:"class"`
	Order          string `json:"order"`
	Family         string `json:"family"`
}

type wireTaxonResp struct {
	Total   int         `json:"total"`
	Results []wireTaxon `json:"results"`
}

type wireDataset struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Url   string `json:"url"`
}

type wireDatasetResp struct {
	Total   int           `json:"total"`
	Results []wireDataset `json:"results"`
}

type wireStats struct {
	Records  int `json:"records"`
	Species  int `json:"species"`
	Taxa     int `json:"taxa"`
	Datasets int `json:"datasets"`
}

// --- public types ---

// Occurrence is a single species sighting record from OBIS.
type Occurrence struct {
	ID             string  `json:"id"              kit:"id"`
	ScientificName string  `json:"scientific_name"`
	Class          string  `json:"class,omitempty"`
	Family         string  `json:"family,omitempty"`
	Genus          string  `json:"genus,omitempty"`
	Latitude       float64 `json:"latitude,omitempty"`
	Longitude      float64 `json:"longitude,omitempty"`
	Depth          float64 `json:"depth,omitempty"`
	Year           int     `json:"year,omitempty"`
	Country        string  `json:"country,omitempty"`
	Dataset        string  `json:"dataset,omitempty"`
	BasisOfRecord  string  `json:"basis_of_record,omitempty"`
}

// Taxon is a taxonomic classification record from OBIS.
type Taxon struct {
	ID             string `json:"id"              kit:"id"`
	ScientificName string `json:"scientific_name"`
	Rank           string `json:"rank,omitempty"`
	Kingdom        string `json:"kingdom,omitempty"`
	Phylum         string `json:"phylum,omitempty"`
	Class          string `json:"class,omitempty"`
	Order          string `json:"order,omitempty"`
	Family         string `json:"family,omitempty"`
}

// Dataset is a contributing data source in OBIS.
type Dataset struct {
	ID    string `json:"id"    kit:"id"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
}

// Stats holds the global OBIS statistics snapshot.
type Stats struct {
	Records  int `json:"records"`
	Species  int `json:"species"`
	Taxa     int `json:"taxa"`
	Datasets int `json:"datasets"`
}

// --- client methods ---

// SearchOccurrences searches for species occurrence records by scientific name.
// It returns up to limit records and the total count of matching records.
func (c *Client) SearchOccurrences(ctx context.Context, scientificName string, limit int) ([]*Occurrence, int, error) {
	q := url.Values{}
	if scientificName != "" {
		q.Set("scientificname", scientificName)
	}
	if limit > 0 {
		q.Set("size", strconv.Itoa(limit))
	}
	endpoint := BaseURL + "/occurrence?" + q.Encode()

	body, err := c.Get(ctx, endpoint)
	if err != nil {
		return nil, 0, err
	}

	var resp wireSearchResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("decode occurrences: %w", err)
	}

	out := make([]*Occurrence, 0, len(resp.Results))
	for _, w := range resp.Results {
		out = append(out, occurrenceFromWire(w))
	}
	return out, resp.Total, nil
}

// GetTaxon looks up taxonomic information for a species by scientific name.
func (c *Client) GetTaxon(ctx context.Context, scientificName string) ([]*Taxon, error) {
	q := url.Values{}
	q.Set("scientificname", scientificName)
	endpoint := BaseURL + "/taxon?" + q.Encode()

	body, err := c.Get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var resp wireTaxonResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode taxon: %w", err)
	}

	out := make([]*Taxon, 0, len(resp.Results))
	for _, w := range resp.Results {
		out = append(out, taxonFromWire(w))
	}
	return out, nil
}

// ListDatasets lists contributing datasets in OBIS, up to limit records.
// It returns the datasets and the total dataset count.
func (c *Client) ListDatasets(ctx context.Context, limit int) ([]*Dataset, int, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("size", strconv.Itoa(limit))
	}
	endpoint := BaseURL + "/dataset?" + q.Encode()

	body, err := c.Get(ctx, endpoint)
	if err != nil {
		return nil, 0, err
	}

	var resp wireDatasetResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("decode datasets: %w", err)
	}

	out := make([]*Dataset, 0, len(resp.Results))
	for _, w := range resp.Results {
		out = append(out, datasetFromWire(w))
	}
	return out, resp.Total, nil
}

// GetStats returns the global OBIS statistics snapshot.
func (c *Client) GetStats(ctx context.Context) (*Stats, error) {
	endpoint := BaseURL + "/statistics"

	body, err := c.Get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var w wireStats
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("decode stats: %w", err)
	}

	return &Stats{
		Records:  w.Records,
		Species:  w.Species,
		Taxa:     w.Taxa,
		Datasets: w.Datasets,
	}, nil
}

// --- conversion helpers ---

func occurrenceFromWire(w wireOccurrence) *Occurrence {
	return &Occurrence{
		ID:             w.ID,
		ScientificName: w.ScientificName,
		Class:          w.Class,
		Family:         w.Family,
		Genus:          w.Genus,
		Latitude:       w.DecimalLat,
		Longitude:      w.DecimalLon,
		Depth:          w.Depth,
		Year:           w.Year,
		Country:        w.Country,
		Dataset:        w.DatasetName,
		BasisOfRecord:  w.BasisOfRecord,
	}
}

func taxonFromWire(w wireTaxon) *Taxon {
	return &Taxon{
		ID:             strconv.Itoa(w.TaxonID),
		ScientificName: w.ScientificName,
		Rank:           w.Rank,
		Kingdom:        w.Kingdom,
		Phylum:         w.Phylum,
		Class:          w.Class,
		Order:          w.Order,
		Family:         w.Family,
	}
}

func datasetFromWire(w wireDataset) *Dataset {
	return &Dataset{
		ID:    w.ID,
		Title: w.Title,
		URL:   w.Url,
	}
}
