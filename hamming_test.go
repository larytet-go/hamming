package hamming

// Try go test -v -bench .

import (
	"testing"

	"github.com/steakknife/hamming"
)

func TestDistance(t *testing.T) {
	s1 := fuzzyHash{0x00, 0x01}
	s2 := fuzzyHash{0x01, 0x01}
	s3 := fuzzyHash{0x00, 0x01}
	s4 := fuzzyHash{0x00, 0x00}
	sibling := closestSibling(s1, []fuzzyHash{s2, s3, s4})
	if sibling.distance != 0 {
		t.Errorf("Found wrong sibling %v", sibling)
	}
	if !sibling.s.isEqual(s1) {
		t.Errorf("Found wrong sibling %v", sibling)
	}
}

type HashStringToFuzzyHashTest struct {
	in         string
	out        fuzzyHash
	raiseError bool
}

var hashStringToFuzzyHashTests = []HashStringToFuzzyHashTest{
	{in: "1122334455667788", out: fuzzyHash{0x1122334455667788}},
	{in: "11e2334455667788", out: fuzzyHash{0x11e2334455667788}},
	{in: "11E2334455667788", out: fuzzyHash{0x11e2334455667788}},
	{in: "11A2334455667788", out: fuzzyHash{0x11a2334455667788}},
	{in: "11b2334455667788", out: fuzzyHash{0x11b2334455667788}},
	{in: "11c2334455667788", out: fuzzyHash{0x11c2334455667788}},
	{in: "11d2334455667788", out: fuzzyHash{0x11d2334455667788}},
	{in: "11e2334455667788", out: fuzzyHash{0x11e2334455667788}},
	{in: "11f2334455667788", out: fuzzyHash{0x11f2334455667788}},
	{in: "11F2334455667788", out: fuzzyHash{0x11f2334455667788}},
	{in: "11K2334455667788", out: fuzzyHash{0x1102334455667788}, raiseError: true},
	{in: "11223344556677881122334455667788", out: fuzzyHash{0x1122334455667788, 0x1122334455667788}},
}

func TestHashStringToFuzzyHash(t *testing.T) {
	for idx, test := range hashStringToFuzzyHashTests {
		fh, err := hashStringToFuzzyHash(test.in)
		if err != nil && !test.raiseError {
			t.Errorf("Test %d failed: %v", idx, err)
		}
		if !fh.isEqual(test.out) && !test.raiseError {
			t.Errorf("Test %d failed: expected %s, got %s", idx, test.out.toString(), fh.toString())
		}
	}
}

func TestFuzzyHashToString(t *testing.T) {
	fh := fuzzyHash{0x1122334455667788, 0x1122334455667788}
	expected := "11223344556677881122334455667788"
	s := fh.toString()
	if s != expected {
		t.Errorf("Expected %s, got %s", expected, s)
	}
}

func BenchmarkClosestSibling(b *testing.B) {
	s, _ := hashStringToFuzzyHash("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")
	s1, _ := hashStringToFuzzyHash("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF1")
	s2, _ := hashStringToFuzzyHash("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF3")
	s3, _ := hashStringToFuzzyHash("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")
	s4, _ := hashStringToFuzzyHash("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF7")
	b.ResetTimer()
	var sibling Sibling
	for i := 0; i < b.N; i++ {
		sibling = closestSibling(s, []fuzzyHash{s1, s2, s3, s4, s1, s2, s3, s4})
	}
	if sibling.distance != 0 {
		b.Errorf("Found wrong sibling %v", sibling)
	}
	if !sibling.s.isEqual(s3) {
		b.Errorf("Found wrong sibling %v", sibling)
	}
}

func BenchmarkClosestSibling2K(b *testing.B) {
	s0, _ := hashStringToFuzzyHash("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")
	var dataSet []fuzzyHash
	for i := 0; i < 2*1000; i++ {
		var s fuzzyHash = make([]uint64, len(s0))
		copy(s, s0)
		dataSet = append(dataSet, s0)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = closestSibling(s0, dataSet)
	}
}

func BenchmarkClosestSibling300K(b *testing.B) {
	s0, _ := hashStringToFuzzyHash("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")
	var dataSet []fuzzyHash
	for i := 0; i < 300*1000; i++ {
		var s fuzzyHash = make([]uint64, len(s0))
		copy(s, s0)
		dataSet = append(dataSet, s0)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = closestSibling(s0, dataSet)
	}
}

func BenchmarkClosestSibling1M(b *testing.B) {
	s0, _ := hashStringToFuzzyHash("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")
	var dataSet []fuzzyHash
	for i := 0; i < 1000*1000; i++ {
		var s fuzzyHash = make([]uint64, len(s0))
		copy(s, s0)
		dataSet = append(dataSet, s0)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = closestSibling(s0, dataSet)
	}
}

func BenchmarkHammingDistance(b *testing.B) {
	d1 := []uint64{
		0x1122334455667788,
		0x1122334455667788,
		0x1122334455667788,
		0x1122334455667788,
		0x1122334455667788,
		0x1122334455667788,
		0x1122334455667788,
		0x1122334455667788}
	d2 := d1
	for i := 0; i < b.N; i++ {
		hamming.Uint64s(d1, d2)
	}
}

func BenchmarkHashStringToFuzzyHash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hashStringToFuzzyHash("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")
	}
}
