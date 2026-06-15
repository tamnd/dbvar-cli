package dbvar

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes dbVar as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/dbvar-cli/dbvar"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// dbvar:// URIs by routing to the operations Register installs. The same
// Domain also builds the standalone dbvar binary (see cli.NewApp), so the
// binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the dbVar driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "dbvar",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "dbvar",
			Short:  "A command line for NCBI dbVar structural variants.",
			Long: `A command line for NCBI dbVar.

dbvar reads structural variation records from the NCBI dbVar database,
which archives genomic structural variants (deletions, duplications,
inversions, insertions) across studies and individual variant calls.
No API key required. 3.2M+ records indexed.`,
			Site: "https://www.ncbi.nlm.nih.gov/dbvar/",
			Repo: "https://github.com/tamnd/dbvar-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "search", Group: "read", List: true,
		Summary: "Search dbVar records by term, variant type, or organism (--limit, --start)",
		Args:    []kit.Arg{{Name: "query", Help: "search query (e.g. deletion, nstd229, human)"}}}, searchRecords)

	kit.Handle(app, kit.OpMeta{Name: "record", Group: "read", Single: true,
		Summary: "Get a single record by numeric UID", URIType: "record", Resolver: true,
		Args: []kit.Arg{{Name: "uid", Help: "dbVar numeric UID"}}}, getRecord)

	kit.Handle(app, kit.OpMeta{Name: "study", Group: "read", List: true,
		Summary: "Search for study records (--limit, --start)",
		Args:    []kit.Arg{{Name: "term", Help: "study search term (e.g. nstd229, deletion, human)"}}}, searchStudies)
}

// newClient builds the dbVar client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
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
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type searchInput struct {
	Query  string  `kit:"arg"          help:"search query"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Start  int     `kit:"flag"         help:"offset for pagination"`
	Client *Client `kit:"inject"`
}

type recordInput struct {
	UID    string  `kit:"arg"    help:"dbVar numeric UID"`
	Client *Client `kit:"inject"`
}

type studyInput struct {
	Term   string  `kit:"arg"          help:"study search term"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Start  int     `kit:"flag"         help:"offset for pagination"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchRecords(ctx context.Context, in searchInput, emit func(*Record) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	records, _, err := in.Client.SearchAndFetch(ctx, in.Query, limit, in.Start)
	if err != nil {
		return err
	}
	for _, r := range records {
		if err := emit(r); err != nil {
			return err
		}
	}
	return nil
}

func getRecord(ctx context.Context, in recordInput, emit func(*Record) error) error {
	r, err := in.Client.GetRecord(ctx, in.UID)
	if err != nil {
		return err
	}
	return emit(r)
}

func searchStudies(ctx context.Context, in studyInput, emit func(*Record) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	// Filter to STUDY object type using eUtils field qualifier.
	query := fmt.Sprintf("%s AND STUDY[obj_type]", in.Term)
	records, _, err := in.Client.SearchAndFetch(ctx, query, limit, in.Start)
	if err != nil {
		return err
	}
	for _, r := range records {
		if err := emit(r); err != nil {
			return err
		}
	}
	return nil
}

// Classify turns any accepted input into the canonical (type, id).
// dbVar numeric UIDs are all digits.
func (Domain) Classify(input string) (string, string, error) {
	s := strings.TrimSpace(input)
	if len(s) > 0 && allDigits(s) {
		return "record", s, nil
	}
	return "", "", errs.Usage("dbvar UIDs are numeric, got %q", input)
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(t, id string) (string, error) {
	switch t {
	case "record":
		return fmt.Sprintf("https://www.ncbi.nlm.nih.gov/dbvar/?term=%s", id), nil
	default:
		return "", errs.Usage("dbvar has no resource type %q", t)
	}
}

// allDigits reports whether s is a non-empty string of ASCII digits.
func allDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
