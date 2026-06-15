package obis

import (
	"context"
	"strconv"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes OBIS as a kit Domain: a driver that a multi-domain host
// (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/obis-cli/obis"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then
// dereferences obis:// URIs by routing to the operations Register installs.
// The same Domain also builds the standalone obis binary (see cli.NewApp),
// so the binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the OBIS driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "obis",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "obis",
			Short:  "Ocean Biodiversity Information System CLI",
			Long: `obis reads public OBIS data over plain HTTPS, shapes it into clean
records, and prints output that pipes into the rest of your tools.
No API key, nothing to run alongside it.

OBIS is the world's largest open-access repository for marine species
occurrence data, with 200M+ records from 6,700+ datasets.`,
			Site: Host,
			Repo: "https://github.com/tamnd/obis-cli",
		},
	}
}

// Register installs the client factory and every OBIS operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "occurrence", Group: "read", List: true,
		Summary: "Search species occurrence records",
		Args:    []kit.Arg{{Name: "scientific-name", Help: "scientific name to search for"}}},
		searchOccurrences)

	kit.Handle(app, kit.OpMeta{Name: "taxon", Group: "read", Single: true,
		Summary: "Look up taxonomic classification for a species", URIType: "taxon", Resolver: true,
		Args: []kit.Arg{{Name: "name", Help: "scientific name"}}},
		getTaxon)

	kit.Handle(app, kit.OpMeta{Name: "datasets", Group: "read", List: true,
		Summary: "List contributing datasets"},
		listDatasets)

	kit.Handle(app, kit.OpMeta{Name: "stats", Group: "read", Single: true,
		Summary: "Show global OBIS statistics"},
		getStats)
}

// newClient builds the OBIS client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type occurrenceIn struct {
	Name   string  `kit:"arg"          help:"scientific name to search for"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type taxonIn struct {
	Name   string  `kit:"arg"    help:"scientific name"`
	Client *Client `kit:"inject"`
}

type datasetsIn struct {
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type statsIn struct {
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchOccurrences(ctx context.Context, in occurrenceIn, emit func(*Occurrence) error) error {
	occs, _, err := in.Client.SearchOccurrences(ctx, in.Name, in.Limit)
	if err != nil {
		return err
	}
	for _, o := range occs {
		if err := emit(o); err != nil {
			return err
		}
	}
	return nil
}

func getTaxon(ctx context.Context, in taxonIn, emit func(*Taxon) error) error {
	taxa, err := in.Client.GetTaxon(ctx, in.Name)
	if err != nil {
		return err
	}
	for _, t := range taxa {
		if err := emit(t); err != nil {
			return err
		}
	}
	return nil
}

func listDatasets(ctx context.Context, in datasetsIn, emit func(*Dataset) error) error {
	datasets, _, err := in.Client.ListDatasets(ctx, in.Limit)
	if err != nil {
		return err
	}
	for _, d := range datasets {
		if err := emit(d); err != nil {
			return err
		}
	}
	return nil
}

func getStats(ctx context.Context, in statsIn, emit func(*Stats) error) error {
	s, err := in.Client.GetStats(ctx)
	if err != nil {
		return err
	}
	return emit(s)
}

// --- Resolver: URI-native string functions ---

// Classify turns any accepted input into (type, id). Any non-empty string
// maps to ("occurrence", input) so it is addressable as a resource URI.
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("obis: empty reference")
	}
	return "occurrence", input, nil
}

// Locate returns the live HTTPS URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "taxon":
		return "https://obis.org/taxon/" + id, nil
	default:
		return "https://obis.org", nil
	}
}

// --- helpers ---

// taxonIDStr converts a numeric taxon ID to a string for Taxon.ID.
func taxonIDStr(id int) string {
	return strconv.Itoa(id)
}
