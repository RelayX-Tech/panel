package geo_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/relayx-tech/panel/internal/geo"
)

func TestParseText(t *testing.T) {
	ds, err := geo.ParseBytes([]byte("cn: example.cn\ncn: example2.cn\n// comment\n\nads: ad.com\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ds.Categories) != 2 {
		t.Fatalf("categories = %d, want 2", len(ds.Categories))
	}
	if got := len(ds.EntriesOf("cn")); got != 2 {
		t.Fatalf("cn entries = %d, want 2", got)
	}
	if got := len(ds.EntriesOf("")); got != 3 {
		t.Fatalf("all entries = %d, want 3", got)
	}
}

func TestParseRejectsEmptyAndBinary(t *testing.T) {
	if _, err := geo.ParseBytes([]byte("// only comments\n")); err == nil {
		t.Fatal("empty dataset should fail")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "geoip.dat")
	if err := os.WriteFile(path, []byte{0x01, 0x02}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := geo.Parse(path); err == nil {
		t.Fatal("binary .dat should be rejected until baseline reader is wired")
	}
}
