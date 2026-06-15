package dbvar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testServer(t *testing.T, mux *http.ServeMux) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 0
	return srv, NewClient(cfg)
}

func TestSearch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/esearch.fcgi", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("db") != "dbvar" {
			http.Error(w, "wrong db", 400)
			return
		}
		json.NewEncoder(w).Encode(wireSearch{
			ESearchResult: struct {
				Count  string   `json:"count"`
				IDList []string `json:"idlist"`
			}{
				Count:  "3261187",
				IDList: []string{"57746190", "57746191", "57746192"},
			},
		})
	})
	_, client := testServer(t, mux)
	ids, total, err := client.Search(context.Background(), "deletion", 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3261187 {
		t.Errorf("total = %d, want 3261187", total)
	}
	if len(ids) != 3 {
		t.Errorf("len = %d, want 3", len(ids))
	}
	if ids[0] != "57746190" {
		t.Errorf("ids[0] = %q, want 57746190", ids[0])
	}
}

func TestFetchRecordsStudy(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/esummary.fcgi", func(w http.ResponseWriter, r *http.Request) {
		rec := wireRecord{
			UID:          "54356918",
			ObjType:      "STUDY",
			St:           "nstd229",
			StudyType:    "Collection",
			VariantCount: 376296,
			Publications: []wirePublication{
				{PMID: 36747810, PublicationName: "Jun et al. 2023"},
			},
			Orgs:       []wireOrg{{TaxID: 9606, Species: "human"}},
			Assemblies: []string{"GRCh38 (hg38)"},
		}
		recBytes, _ := json.Marshal(rec)
		result := map[string]json.RawMessage{
			"uids":     json.RawMessage(`["54356918"]`),
			"54356918": recBytes,
		}
		json.NewEncoder(w).Encode(map[string]any{"result": result})
	})
	_, client := testServer(t, mux)
	records, err := client.FetchRecords(context.Background(), []string{"54356918"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("len = %d, want 1", len(records))
	}
	r := records[0]
	if r.ID != "54356918" {
		t.Errorf("ID = %q, want 54356918", r.ID)
	}
	if r.Type != "STUDY" {
		t.Errorf("Type = %q, want STUDY", r.Type)
	}
	if r.StudyAccession != "nstd229" {
		t.Errorf("StudyAccession = %q, want nstd229", r.StudyAccession)
	}
	if r.VariantCount != 376296 {
		t.Errorf("VariantCount = %d, want 376296", r.VariantCount)
	}
	if r.Organism != "human" {
		t.Errorf("Organism = %q, want human", r.Organism)
	}
	if len(r.Publications) != 1 || r.Publications[0] != "Jun et al. 2023" {
		t.Errorf("Publications = %v, want [Jun et al. 2023]", r.Publications)
	}
	if len(r.Assemblies) != 1 || r.Assemblies[0] != "GRCh38 (hg38)" {
		t.Errorf("Assemblies = %v, want [GRCh38 (hg38)]", r.Assemblies)
	}
}

func TestFetchRecordsVariant(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/esummary.fcgi", func(w http.ResponseWriter, r *http.Request) {
		rec := wireRecord{
			UID:      "57746190",
			ObjType:  "VARIANT",
			St:       "nstd232",
			Sv:       "nsv7905151",
			Organism: "human",
			TaxID:    "9606",
		}
		recBytes, _ := json.Marshal(rec)
		result := map[string]json.RawMessage{
			"uids":     json.RawMessage(`["57746190"]`),
			"57746190": recBytes,
		}
		json.NewEncoder(w).Encode(map[string]any{"result": result})
	})
	_, client := testServer(t, mux)
	records, err := client.FetchRecords(context.Background(), []string{"57746190"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("len = %d, want 1", len(records))
	}
	v := records[0]
	if v.ID != "57746190" {
		t.Errorf("ID = %q, want 57746190", v.ID)
	}
	if v.Type != "VARIANT" {
		t.Errorf("Type = %q, want VARIANT", v.Type)
	}
	if v.StudyAccession != "nstd232" {
		t.Errorf("StudyAccession = %q, want nstd232", v.StudyAccession)
	}
	if v.VariantAccession != "nsv7905151" {
		t.Errorf("VariantAccession = %q, want nsv7905151", v.VariantAccession)
	}
	if v.Organism != "human" {
		t.Errorf("Organism = %q, want human", v.Organism)
	}
}

func TestGetRecord(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/esummary.fcgi", func(w http.ResponseWriter, r *http.Request) {
		rec := wireRecord{UID: "42", ObjType: "STUDY", St: "nstd42", StudyType: "Case-Control"}
		recBytes, _ := json.Marshal(rec)
		result := map[string]json.RawMessage{
			"uids": json.RawMessage(`["42"]`),
			"42":   recBytes,
		}
		json.NewEncoder(w).Encode(map[string]any{"result": result})
	})
	_, client := testServer(t, mux)
	r, err := client.GetRecord(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "42" {
		t.Errorf("ID = %q, want 42", r.ID)
	}
	if r.StudyAccession != "nstd42" {
		t.Errorf("StudyAccession = %q, want nstd42", r.StudyAccession)
	}
}

func TestFetchRecordsEmpty(t *testing.T) {
	_, client := testServer(t, http.NewServeMux())
	records, err := client.FetchRecords(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if records != nil {
		t.Error("expected nil records for empty input")
	}
}
