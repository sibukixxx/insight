package service

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestFingerprintMatchesSHA256OfSortedKeyCompactJSONWithoutHTMLEscaping(t *testing.T) {
	got, err := Fingerprint(map[string]any{"b": "<&>", "a": 1})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(`{"a":1,"b":"<&>"}`))
	if want := "sha256:" + hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("Fingerprint = %s, want %s", got, want)
	}
}

func TestFingerprintIgnoresStructFieldOrderAndMapInsertionOrder(t *testing.T) {
	type ab struct {
		A string `json:"a"`
		B string `json:"b"`
	}
	type ba struct {
		B string `json:"b"`
		A string `json:"a"`
	}
	x, _ := Fingerprint(ab{A: "1", B: "2"})
	y, _ := Fingerprint(ba{B: "2", A: "1"})
	z, _ := Fingerprint(map[string]string{"b": "2", "a": "1"})
	if x != y || y != z {
		t.Fatalf("fingerprints differ: %s %s %s", x, y, z)
	}
}

func TestFingerprintTreatsNilAndEmptyCollectionsAsAbsent(t *testing.T) {
	type holder struct {
		List []string          `json:"list"`
		Map  map[string]string `json:"map"`
		Ptr  *string           `json:"ptr"`
		Name string            `json:"name"`
	}
	nilValues, _ := Fingerprint(holder{Name: "x"})
	emptyValues, _ := Fingerprint(holder{List: []string{}, Map: map[string]string{}, Name: "x"})
	absent, _ := Fingerprint(map[string]string{"name": "x"})
	if nilValues != emptyValues || emptyValues != absent {
		t.Fatalf("nil/empty/absent differ: %s %s %s", nilValues, emptyValues, absent)
	}
}

func TestFingerprintKeepsSliceOrderAndDistinguishesValues(t *testing.T) {
	ab, _ := Fingerprint([]string{"a", "b"})
	ba, _ := Fingerprint([]string{"b", "a"})
	if ab == ba {
		t.Fatal("slice order is meaningful and must change the fingerprint")
	}
}
