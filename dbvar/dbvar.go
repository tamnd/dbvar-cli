// Package dbvar is the library behind the dbvar command line:
// the HTTP client, request shaping, and the typed data models for NCBI dbVar.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that the eUtils API throws under load.
package dbvar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Host is the eUtils hostname this client talks to, and the host the URI
// driver in domain.go claims.
const Host = "eutils.ncbi.nlm.nih.gov"

const baseURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"

// Config holds the runtime settings for the dbVar client.
type Config struct {
	BaseURL   string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
	UserAgent string
}

// DefaultConfig returns a Config with sensible defaults: 400ms rate limit,
// 3 retries, and a 30s timeout.
func DefaultConfig() Config {
	return Config{
		BaseURL:   baseURL,
		Rate:      400 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
		UserAgent: "dbvar-cli/0.1.0 (github.com/tamnd/dbvar-cli)",
	}
}

// Client talks to NCBI eUtils for dbVar records.
type Client struct {
	cfg  Config
	http *http.Client
	last time.Time
}

// NewClient returns a Client using the given Config.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *Client) wait() {
	if c.cfg.Rate > 0 {
		if since := time.Since(c.last); since < c.cfg.Rate {
			time.Sleep(c.cfg.Rate - since)
		}
	}
	c.last = time.Now()
}

func (c *Client) get(ctx context.Context, rawURL string, out any) error {
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			d := time.Duration(attempt) * 500 * time.Millisecond
			if d > 5*time.Second {
				d = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
			}
		}
		c.wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", c.cfg.UserAgent)
		resp, err := c.http.Do(req)
		if err != nil {
			if attempt < c.cfg.Retries {
				continue
			}
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			if attempt < c.cfg.Retries {
				continue
			}
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return fmt.Errorf("all retries exhausted")
}

// --- wire types (unexported) ---

type wireSearch struct {
	ESearchResult struct {
		Count  string   `json:"count"`
		IDList []string `json:"idlist"`
	} `json:"esearchresult"`
}

type wirePublication struct {
	PMID            int    `json:"pmid"`
	PublicationName string `json:"publication_name"`
}

type wireOrg struct {
	TaxID   int    `json:"tax_id"`
	Species string `json:"species"`
}

type wireRecord struct {
	UID          string           `json:"uid"`
	ObjType      string           `json:"obj_type"`
	St           string           `json:"st"`  // study accession (nstd...)
	Sv           string           `json:"sv"`  // variant accession (nsv...)
	StudyType    string           `json:"study_type"`
	VariantCount int              `json:"variant_count"`
	Organism     string           `json:"organism"`
	TaxID        string           `json:"tax_id"`
	Publications []wirePublication `json:"dbvarpublicationlist"`
	Orgs         []wireOrg        `json:"dbvarstudyorglist"`
	Assemblies   []string         `json:"dbvarsubmittedassemblylist"`
	VariantTypes []string         `json:"dbvarvarianttypelist"`
	ClinSig      []string         `json:"dbvarclinicalsignificancelist"`
}

type wireSummary struct {
	Result map[string]json.RawMessage `json:"result"`
}

// --- public types ---

// Record is a single dbVar record, either a study (Type=="STUDY") or a
// structural variant (Type=="VARIANT").
type Record struct {
	ID               string   `json:"id"                          kit:"id"`
	Type             string   `json:"type"`
	StudyAccession   string   `json:"study_accession,omitempty"`
	VariantAccession string   `json:"variant_accession,omitempty"`
	StudyType        string   `json:"study_type,omitempty"`
	VariantCount     int      `json:"variant_count,omitempty"`
	Organism         string   `json:"organism,omitempty"`
	Assemblies       []string `json:"assemblies,omitempty"`
	VariantTypes     []string `json:"variant_types,omitempty"`
	ClinSig          []string `json:"clinical_significance,omitempty"`
	Publications     []string `json:"publications,omitempty"` // "Author Year" from publication_name
}

func toRecord(w wireRecord) *Record {
	var pubs []string
	for _, p := range w.Publications {
		if p.PublicationName != "" {
			pubs = append(pubs, p.PublicationName)
		}
	}

	// organism: prefer the org list species, fall back to organism field
	organism := w.Organism
	if organism == "" && len(w.Orgs) > 0 {
		organism = w.Orgs[0].Species
	}

	// study_accession is always the St field; variant_accession is only set for VARIANTs
	r := &Record{
		ID:             w.UID,
		Type:           w.ObjType,
		StudyAccession: w.St,
		StudyType:      w.StudyType,
		VariantCount:   w.VariantCount,
		Organism:       organism,
		Assemblies:     nilIfEmpty(w.Assemblies),
		VariantTypes:   nilIfEmpty(w.VariantTypes),
		ClinSig:        nilIfEmpty(w.ClinSig),
		Publications:   nilIfEmpty(pubs),
	}
	if w.ObjType == "VARIANT" {
		r.VariantAccession = w.Sv
	}
	return r
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

// Search searches dbVar for records matching the query and returns IDs and
// the total hit count.
func (c *Client) Search(ctx context.Context, query string, limit, start int) ([]string, int, error) {
	u := fmt.Sprintf("%s/esearch.fcgi?db=dbvar&term=%s&retmax=%d&retstart=%d&retmode=json",
		c.cfg.BaseURL, url.QueryEscape(query), limit, start)
	var w wireSearch
	if err := c.get(ctx, u, &w); err != nil {
		return nil, 0, err
	}
	count := 0
	fmt.Sscanf(w.ESearchResult.Count, "%d", &count)
	return w.ESearchResult.IDList, count, nil
}

// FetchRecords fetches record details for the given IDs (up to ~500 per
// call; callers should batch if needed).
func (c *Client) FetchRecords(ctx context.Context, ids []string) ([]*Record, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	u := fmt.Sprintf("%s/esummary.fcgi?db=dbvar&id=%s&retmode=json",
		c.cfg.BaseURL, strings.Join(ids, ","))
	var w wireSummary
	if err := c.get(ctx, u, &w); err != nil {
		return nil, err
	}
	rawUIDs, ok := w.Result["uids"]
	if !ok {
		return nil, fmt.Errorf("no uids in esummary response")
	}
	var uids []string
	if err := json.Unmarshal(rawUIDs, &uids); err != nil {
		return nil, err
	}
	var records []*Record
	for _, uid := range uids {
		raw, ok := w.Result[uid]
		if !ok {
			continue
		}
		var wr wireRecord
		if err := json.Unmarshal(raw, &wr); err != nil {
			continue
		}
		records = append(records, toRecord(wr))
	}
	return records, nil
}

// GetRecord fetches a single dbVar record by its numeric UID.
func (c *Client) GetRecord(ctx context.Context, uid string) (*Record, error) {
	records, err := c.FetchRecords(ctx, []string{uid})
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("record %s not found", uid)
	}
	return records[0], nil
}

// SearchAndFetch searches dbVar and returns full Record objects.
func (c *Client) SearchAndFetch(ctx context.Context, query string, limit, start int) ([]*Record, int, error) {
	ids, total, err := c.Search(ctx, query, limit, start)
	if err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return nil, total, nil
	}
	records, err := c.FetchRecords(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	return records, total, nil
}
