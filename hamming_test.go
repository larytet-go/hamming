package hamming

// Try go test -v -bench .

import (
	"bufio"
	"flag"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/steakknife/hamming"
)

// There is a "real-life" benchmark which requires a data set
// The data set file contains one or more hashes separated by a newline
// Tips.
// Dump from the SQL file
// create database hashes
// use hashes;
// source hashes.sql ;
// select HEX(code) from hashes  INTO OUTFILE '/var/lib/mysql-files/hashes.0.csv' LINES TERMINATED BY '\n';
// Remove the leading and trailing brackets
// cat /var/lib/mysql-files/hashes.0.csv | sed 's/^7B\(.*\)7D$/\1/' > hashes.0.clean.csv
var dataSetFilenameFlag = flag.String("dataset", "", "File containing the data set to check")
var dataSetMaximumDistanceFlag = flag.String("distance", "0", "Maximum hamming distance")

const allFsHash = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"
const allZerosHash = "0000000000000000000000000000000000000000000000000000000000000000"

var allZerosHashBin = FuzzyHash{0x00, 0x00, 0x00, 0x00}

func TestDistance(t *testing.T) {
	s1 := FuzzyHash{0x00, 0x01}
	s2 := FuzzyHash{0x01, 0x01}
	s3 := FuzzyHash{0x00, 0x01}
	s4 := FuzzyHash{0x00, 0x00}
	sibling := closestSibling(s1, []FuzzyHash{s2, s3, s4})
	if sibling.distance != 0 {
		t.Errorf("Found wrong sibling %v", sibling)
	}
	if !sibling.s.IsEqual(s1) {
		t.Errorf("Found wrong sibling %v", sibling)
	}
}

type HashStringToFuzzyHashTest struct {
	in         string
	out        FuzzyHash
	raiseError bool
}

var hashStringToFuzzyHashTests = []HashStringToFuzzyHashTest{
	{in: "1122334455667788", out: FuzzyHash{0x1122334455667788}},
	{in: "11e2334455667788", out: FuzzyHash{0x11e2334455667788}},
	{in: "11E2334455667788", out: FuzzyHash{0x11e2334455667788}},
	{in: "11A2334455667788", out: FuzzyHash{0x11a2334455667788}},
	{in: "11b2334455667788", out: FuzzyHash{0x11b2334455667788}},
	{in: "11c2334455667788", out: FuzzyHash{0x11c2334455667788}},
	{in: "11d2334455667788", out: FuzzyHash{0x11d2334455667788}},
	{in: "11e2334455667788", out: FuzzyHash{0x11e2334455667788}},
	{in: "11f2334455667788", out: FuzzyHash{0x11f2334455667788}},
	{in: "11F2334455667788", out: FuzzyHash{0x11f2334455667788}},
	{in: "11K2334455667788", out: FuzzyHash{0x1102334455667788}, raiseError: true},
	{in: "0000000000000011", out: FuzzyHash{0x11}},
	{in: "11223344556677881122334455667788", out: FuzzyHash{0x1122334455667788, 0x1122334455667788}},
	{in: "11223344056677800022334455667088", out: FuzzyHash{0x1122334405667780, 0x0022334455667088}},
	{in: "00000000000000000000000000000011", out: FuzzyHash{0x00, 0x11}},
}

func TestHashStringToFuzzyHash(t *testing.T) {
	for testID, test := range hashStringToFuzzyHashTests {
		fh, err := HashStringToFuzzyHash(test.in)
		if err != nil && !test.raiseError {
			t.Errorf("Test %d failed: %v", testID, err)
		}
		if !fh.IsEqual(test.out) && !test.raiseError {
			t.Errorf("Test %d failed: expected %s, got %s", testID, test.out.ToString(), fh.ToString())
		}
		if strings.ToUpper(fh.ToString()) != strings.ToUpper(test.in) && !test.raiseError {
			t.Errorf("Test %d failed: expected %s, got %s", testID, test.in, fh.ToString())
		}
	}
}

func TestFuzzyHashAnd(t *testing.T) {
	fh := FuzzyHash{0x3031323334353637, 0x3736353433323130}
	mask := uint64(0xFF01)
	expected := fh[1] & mask

	v := fh.and(mask)
	if v != expected {
		t.Errorf("Expected %x, got %x", expected, v)
	}
}

type HashFuzzyHashRshTest struct {
	in  string
	s   uint64
	out string
}

var hashFuzzyHashRshTests = []HashFuzzyHashRshTest{
	// https://defuse.ca/big-number-calculator.htm
	{in: "11223344556677881122334455667788", s: 1, out: "089119a22ab33bc4089119a22ab33bc4"},
	{in: "11223344556677881122334455667788", s: 2, out: "04488cd115599de204488cd115599de2"},
	{in: "11223344556677881122334455667788", s: 3, out: "022446688aaccef1022446688aaccef1"},
	{in: "11223344556677881122334455667788", s: 4, out: "01122334455667788112233445566778"},
	{in: "11223344556677881122334455667788", s: 5, out: "0089119a22ab33bc4089119a22ab33bc"},
	{in: "11223344556677881122334455667788", s: 6, out: "004488cd115599de204488cd115599de"},
	{in: "11223344556677881122334455667788", s: 7, out: "0022446688aaccef1022446688aaccef"},
	{in: "11223344556677881122334455667788", s: 8, out: "00112233445566778811223344556677"},
	{in: "11223344556677881122334455667788", s: 9, out: "00089119a22ab33bc4089119a22ab33b"},
	{in: "11223344556677881122334455667788", s: 10, out: "0004488cd115599de204488cd115599d"},
	{in: "11223344556677881122334455667788", s: 11, out: "00022446688aaccef1022446688aacce"},
}

func TestFuzzyHashRsh(t *testing.T) {
	for testID, test := range hashFuzzyHashRshTests {
		fh, _ := HashStringToFuzzyHash(test.in)
		fh.rsh(test.s)
		if fh.ToString() != test.out {
			t.Errorf("Test %d failed: expected %s, got %s", testID, test.out, fh.ToString())
		}
	}
}

type GenerateBitCombinationsTest struct {
	value        uint64
	combinations [][]int
	result       []uint64
}

var generateBitCombinationsTests = []GenerateBitCombinationsTest{
	{value: 0x1122334455667788, combinations: [][]int{{0, 1}, {2, 3}, {3, 4, 5}}, result: []uint64{0x00, 0x02, 0x01}},
}

func equalUint64(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
func TestGenerateBitCombinations(t *testing.T) {
	for testID, test := range generateBitCombinationsTests {
		result := generateBitCombinations(test.value, test.combinations)
		if !equalUint64(result, test.result) {
			t.Errorf("Test %d failed: expected %v, got %v", testID, test.result, result)
		}
	}
}

func TestFuzzyHashToKey(t *testing.T) {
	fh := FuzzyHash{0x3031323334353637, 0x3736353433323130}
	expected := "\x37\x36\x35\x34\x33\x32\x31\x30\x30\x31\x32\x33\x34\x35\x36\x37"
	s := fh.toKey()
	if len(s) != len(expected) {
		t.Errorf("Expected %d bytes, got %d bytes", len(expected), len(s))
	}
	if s != expected {
		t.Errorf("Expected '%s', got %s", expected, s)
	}
}

func TestFuzzyHashToString(t *testing.T) {
	fh := FuzzyHash{0x1122334455667788, 0x1122334455667788}
	expected := "11223344556677881122334455667788"
	s := fh.ToString()
	if s != expected {
		t.Errorf("Expected %s, got %s", expected, s)
	}
}

type HammingAddTest struct {
	hashSize    int
	maxDistance int
	hashes      []string
}

var hammingAddTests = []HammingAddTest{
	// https://defuse.ca/big-number-calculator.htm
	{hashSize: 256, maxDistance: 35, hashes: []string{
		"0000000000000000000000000000000000000000000000000000000000000000",
	},
	},
}

func TestHammingAdd(t *testing.T) {
	for testID, test := range hammingAddTests {
		h, err := New(test.hashSize, test.maxDistance)
		if err != nil {
			t.Errorf("Test %d failed: %v", testID, err)
		}
		for _, hash := range test.hashes {
			fh, _ := HashStringToFuzzyHash(hash)
			if !h.add(fh) {
				t.Errorf("Test %d failed", testID)
			}
			if !h.Contains(fh) {
				t.Errorf("Test %d failed", testID)
			}
			if !h.remove(fh) {
				t.Errorf("Test %d failed", testID)
			}
		}
	}
}

type HammingDistanceTest struct {
	hashSize    int
	maxDistance int
	hashes      []string
	sampleHash  string
	bestSibling Sibling
}

var hammingDistanceTests = []HammingDistanceTest{
	// https://defuse.ca/big-number-calculator.htm
	{hashSize: 256, maxDistance: 35, hashes: []string{ // 0
		allZerosHash,
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000011",
		"0000000000000000000000000000000000000000000000000000000000000111",
		"0000000000000000000000000000000000000000000000000000000000001111",
		"0000000000000000000000000000000000000000000000000000000000011111",
		"0000000000000000000000000000000000000000000000000000000000111111",
	},
		sampleHash:  allZerosHash,
		bestSibling: Sibling{distance: 0, s: allZerosHashBin},
	},
	{hashSize: 256, maxDistance: 35, hashes: []string{ // 1
		allZerosHash,
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000011",
		"0000000000000000000000000000000000000000000000000000000000000111",
		"0000000000000000000000000000000000000000000000000000000000001111",
		"0000000000000000000000000000000000000000000000000000000000011111",
		"0000000000000000000000000000000000000000000000000000000000111111",
	},
		sampleHash:  "0000000000000000000000000000000000000000000000000000000000111111",
		bestSibling: Sibling{distance: 0, s: FuzzyHash{0x00, 0x00, 0x00, 0x111111}},
	},
	{hashSize: 256, maxDistance: 35, hashes: []string{ // 2
		allZerosHash,
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000011",
		"0000000000000000000000000000000000000000000000000000000000000111",
		"0000000000000000000000000000000000000000000000000000000000001111",
		"0000000000000000000000000000000000000000000000000000000000011111",
		"0000000000000000000000000000000000000000000000000000000000111111",
	},
		sampleHash:  "0000000000000000000000000000000000000000000000000000000001111111",
		bestSibling: Sibling{distance: 1, s: FuzzyHash{0x00, 0x00, 0x00, 0x111111}},
	},
	{hashSize: 256, maxDistance: 35, hashes: []string{ // 3
		allZerosHash,
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000011",
		"0000000000000000000000000000000000000000000000000000000000000111",
		"0000000000000000000000000000000000000000000000000000000000001111",
		"0000000000000000000000000000000000000000000000000000000000011111",
		"0000000000000000000000000000000000000000000000000000000000111111",
	},
		sampleHash:  "0000000000000000000001111111111111111111111111111111111111111111",
		bestSibling: Sibling{distance: 37, s: FuzzyHash{0x00, 0x00, 0x00, 0x111111}},
	},
	{hashSize: 256, maxDistance: 35, hashes: []string{ // 4
		allZerosHash,
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000011",
		"0000000000000000000000000000000000000000000000000000000000000111",
		"0000000000000000000000000000000000000000000000000000000000001111",
		"0000000000000000000000000000000000000000000000000000000000011111",
		"0000000000000000000000000000000000000000000000000000000000111111",
	},
		sampleHash:  "1000000000000000000000000000000000000000000000000000000000100001",
		bestSibling: Sibling{distance: 2, s: FuzzyHash{0x00, 0x00, 0x00, 0x1}},
	},
	{hashSize: 256, maxDistance: 35, hashes: []string{ // 5
		allZerosHash,
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000011",
		"0000000000000000000000000000000000000000000000000000000000000111",
		"0000000000000000000000000000000000000000000000000000000000001111",
		"0000000000000000000000000000000000000000000000000000000000011111",
		"0000000000000000000000000000000000000000000000000000000000111111",
	},
		sampleHash:  "1000000000000000000000000000000000000000000000000000000000000001",
		bestSibling: Sibling{distance: 1, s: FuzzyHash{0x00, 0x00, 0x00, 0x1}},
	},
	{hashSize: 256, maxDistance: 35, hashes: []string{ // 6
		allZerosHash,
		"1000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000011",
		"0000000000000000000000000000000000000000000000000000000000000111",
		"0000000000000000000000000000000000000000000000000000000000001111",
		"0000000000000000000000000000000000000000000000000000000000011111",
		"0000000000000000000000000000000000000000000000000000000000111111",
	},
		sampleHash:  "1000000000000000000000000000000000000000000000000000000000000001",
		bestSibling: Sibling{distance: 0, s: FuzzyHash{0x1000000000000000, 0x00, 0x00, 0x1}},
	},
	{hashSize: 256, maxDistance: 35, hashes: []string{ // 7
		allZerosHash,
		"1000000000000000000000000000000000000000000000000000000000000001",
		"8800000000000000000000000000000000000000000000000000000000000001",
		"8000000000000000000000000000000000000000000000000000000000000001",
	},
		sampleHash:  "8000000000000000000000000000000000000000000000000000000000000001",
		bestSibling: Sibling{distance: 0, s: FuzzyHash{0x8000000000000000, 0x00, 0x00, 0x1}},
	},
}

func TestHammingDistance(t *testing.T) {
	for testID, test := range hammingDistanceTests {
		h, err := New(test.hashSize, test.maxDistance)
		if err != nil {
			t.Errorf("Test %d failed: %v", testID, err)
		}
		for _, hash := range test.hashes {
			fh, _ := HashStringToFuzzyHash(hash)
			if !h.add(fh) {
				t.Errorf("Test %d failed", testID)
			}
			if !h.Contains(fh) {
				t.Errorf("Test %d failed", testID)
			}
		}
		fh, _ := HashStringToFuzzyHash(test.sampleHash)
		bestSibling := h.ShortestDistance(fh)
		if bestSibling.distance != test.bestSibling.distance {
			t.Errorf("Test %d failed: expected %d got %d", testID, test.bestSibling.distance, bestSibling.distance)
		}
		if !bestSibling.s.IsEqual(test.bestSibling.s) {
			t.Errorf("Test %d failed: expected %s got %s", testID, test.bestSibling.s.ToString(), bestSibling.s.ToString())
		}
	}
}

func TestHammingDup(t *testing.T) {
	fh, _ := HashStringToFuzzyHash(allFsHash)
	h, _ := New(256, 35)
	h.add(fh)
	sibling := h.ShortestDistance(fh)
	if sibling.distance != 0 || !sibling.s.IsEqual(fh) {
		t.Errorf("Failed to find sibling: got distance %d, hash %s", sibling.distance, sibling.s.ToString())
	}
	h = h.Dup()
	sibling = h.ShortestDistance(fh)
	if sibling.distance != 0 || !sibling.s.IsEqual(fh) {
		t.Errorf("Failed to find sibling: got distance %d, hash %s", sibling.distance, sibling.s.ToString())
	}
}

var realDataTest *H

// Try "go test -v -bench . -dataset hashes.csv -distance 35"
func TestLoadRealData(t *testing.T) {
	flag.Parse()
	dataSetFilename := *dataSetFilenameFlag
	if dataSetFilename == "" {
		return
	}
	dataSetMaximumDistance, err := strconv.Atoi(*dataSetMaximumDistanceFlag)
	if err != nil {
		return
	}
	if dataSetMaximumDistance == 0 {
		t.Errorf("Missig maximum distance value flag '-distance'")
	}
	dataFile, err := os.Open(dataSetFilename)
	if err != nil {
		t.Errorf("Failed to open file '%s' %v", dataSetFilename, err)
	}
	defer dataFile.Close()
	reader := bufio.NewReader(dataFile)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if realDataTest == nil {
			hashSize := 8 * len(line) / 2 // bits
			realDataTest, err = New(hashSize, dataSetMaximumDistance)
			if err != nil {
				t.Errorf("Failed to create multi-index file '%s' %v", dataSetFilename, err)
			}
		}
		fh, err := HashStringToFuzzyHash(string(line))
		if err != nil {
			t.Errorf("Failed to parse hash string %s from file '%s' %v", line, dataSetFilename, err)
		}
		realDataTest.add(fh)
	}
	hashesCount := len(realDataTest.hashes)
	t.Logf("Loaded %d hashes from the file '%s'", hashesCount, dataSetFilename)
	var sibling Sibling
	for i := 0; i < hashesCount; i++ {
		fh := realDataTest.hashes[i]
		sibling = realDataTest.ShortestDistance(fh)
		if sibling.distance != 0 || !sibling.s.IsEqual(fh) {
			t.Errorf("Wrong sibiling found %v, distance should", sibling)
		}
	}
	t.Logf("Lookup of hashes completed. Last sibling is %s, distance %d", sibling.s.ToString(), sibling.distance)
}

// XorShift1024Star holds the state required by XorShift1024Star generator.
// I need a fast&dirty pseudo random generator for benchmarking
// This is from https://github.com/vpxyz/xorshift/blob/master/xorshift1024star/xorshift1024star.go
// The custom PRG shaves is cheaper by 20ns than Golang's math rand.Uint64()
type XorShift1024Star struct {
	// The state must be seeded with a nonzero value. Require 16 64-bit unsigned values.
	// The state must be seeded so that it is not everywhere zero. If you have a 64-bit seed,
	// we suggest to seed a xorshift64* generator and use its output to fill s .
	s [16]uint64
	p int
}

// Uint64 returns the next pseudo random number generated, before start you must provvide seed.
func (x *XorShift1024Star) Uint64() uint64 {
	xpnew := (x.p + 1) & 15
	s0 := x.s[x.p]
	s1 := x.s[xpnew]

	s1 ^= s1 << 31 // a
	tmp := s1 ^ s0 ^ (s1 >> 11) ^ (s0 >> 30)

	// update the generator state
	x.s[xpnew] = tmp
	x.p = xpnew

	return tmp * uint64(1181783497276652981)
}

func (x *XorShift1024Star) Init() {
	rand.Seed(999)
	for i := 0; i < len(x.s); i++ {
		x.s[i] = rand.Uint64()
	}
	x.p = 0
}

func benchmarkRealDataSet(count int, b *testing.B) {
	hashesCount := len(realDataTest.hashes)
	xs := &XorShift1024Star{}
	xs.Init()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for k := 0; k < count; k++ {
			testHashIndex := xs.Uint64() % uint64(hashesCount)
			fh := realDataTest.hashes[testHashIndex]
			realDataTest.ShortestDistance(fh)
		}
	}
}

func BenchmarkRealDataSet(b *testing.B) {
	if realDataTest == nil {
		return
	}
	benchmarkRealDataSet(1, b)
}

func BenchmarkRealDataSet100K(b *testing.B) {
	if realDataTest == nil {
		return
	}
	benchmarkRealDataSet(100*1000, b)
}

func BenchmarkRealDataSet1M(b *testing.B) {
	if realDataTest == nil {
		return
	}
	benchmarkRealDataSet(1000*1000, b)
}

func BenchmarkHammingAdd(b *testing.B) {
	h, _ := New(256, 35)
	fh, _ := HashStringToFuzzyHash(allFsHash)
	for i := 0; i < b.N; i++ {
		h.add(fh)
		h.RemoveAll()
	}
}

func BenchmarkFuzzyHashToKey(b *testing.B) {
	fh, _ := HashStringToFuzzyHash(allFsHash)
	for i := 0; i < b.N; i++ {
		fh.toKey()
	}
}

func BenchmarkFuzzyHashToString(b *testing.B) {
	fh, _ := HashStringToFuzzyHash(allFsHash)
	for i := 0; i < b.N; i++ {
		fh.ToString()
	}
}

func BenchmarkClosestSibling(b *testing.B) {
	s, _ := HashStringToFuzzyHash(allFsHash)
	s1, _ := HashStringToFuzzyHash("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF1")
	s2, _ := HashStringToFuzzyHash("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF3")
	s3, _ := HashStringToFuzzyHash(allFsHash)
	s4, _ := HashStringToFuzzyHash("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF7")
	b.ResetTimer()
	var sibling Sibling
	for i := 0; i < b.N; i++ {
		sibling = closestSibling(s, []FuzzyHash{s1, s2, s3, s4, s1, s2, s3, s4})
	}
	if sibling.distance != 0 {
		b.Errorf("Found wrong sibling %v", sibling)
	}
	if !sibling.s.IsEqual(s3) {
		b.Errorf("Found wrong sibling %v", sibling)
	}
}

func benchmarkClosestSiblingInSet(setSize int, b *testing.B) {
	s0, _ := HashStringToFuzzyHash(allFsHash)
	var dataSet []FuzzyHash
	for i := 0; i < setSize; i++ {
		s := s0.Dup() // Different address to force data cache miss
		dataSet = append(dataSet, s)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = closestSibling(s0, dataSet)
	}
}

func BenchmarkClosestSibling2K(b *testing.B) {
	benchmarkClosestSiblingInSet(2*1000, b)
}

func BenchmarkClosestSibling100K(b *testing.B) {
	benchmarkClosestSiblingInSet(100*1000, b)
}

func BenchmarkClosestSibling1M(b *testing.B) {
	benchmarkClosestSiblingInSet(1000*1000, b)
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
		HashStringToFuzzyHash(allFsHash)
	}
}
