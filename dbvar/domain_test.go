package dbvar

import (
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string functions,
// which need no network. The client's HTTP behaviour is covered in dbvar_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "dbvar" {
		t.Errorf("Scheme = %q, want dbvar", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "dbvar" {
		t.Errorf("Identity.Binary = %q, want dbvar", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	typ, id, err := Domain{}.Classify("57746190")
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if typ != "record" {
		t.Errorf("type = %q, want record", typ)
	}
	if id != "57746190" {
		t.Errorf("id = %q, want 57746190", id)
	}
}

func TestClassifyInvalid(t *testing.T) {
	_, _, err := Domain{}.Classify("nstd229")
	if err == nil {
		t.Fatal("expected error for non-numeric input")
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("record", "57746190")
	if err != nil {
		t.Fatalf("Locate error: %v", err)
	}
	want := "https://www.ncbi.nlm.nih.gov/dbvar/?term=57746190"
	if got != want {
		t.Errorf("Locate = %q, want %q", got, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("study", "nstd229")
	if err == nil {
		t.Fatal("expected error for unknown resource type")
	}
}
