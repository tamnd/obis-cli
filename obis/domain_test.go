package obis

import (
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string
// functions. The client's HTTP behaviour is covered in obis_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "obis" {
		t.Errorf("Scheme = %q, want obis", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "obis" {
		t.Errorf("Identity.Binary = %q, want obis", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in, typ, id string
	}{
		{"Tursiops truncatus", "occurrence", "Tursiops truncatus"},
		{"Gadus morhua", "occurrence", "Gadus morhua"},
		{"any string at all", "occurrence", "any string at all"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify(\"\") should return an error")
	}
}

func TestLocate(t *testing.T) {
	tests := []struct {
		uriType string
		id      string
		want    string
	}{
		{"taxon", "137094", "https://obis.org/taxon/137094"},
		{"occurrence", "occ-1", "https://obis.org"},
		{"dataset", "ds-1", "https://obis.org"},
	}
	for _, tc := range tests {
		got, err := Domain{}.Locate(tc.uriType, tc.id)
		if err != nil {
			t.Errorf("Locate(%q, %q) error: %v", tc.uriType, tc.id, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Locate(%q, %q) = %q, want %q", tc.uriType, tc.id, got, tc.want)
		}
	}
}
