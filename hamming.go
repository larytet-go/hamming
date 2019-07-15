// Package hamming implelements HTTP server calculating hamming distance between a given fuzzy hash sample
// and the data set
// Examples:
// http://127.0.0.1/distance?hash=D3790791D1078229DFDD38DF6024E72430B91170F13333660439B2AC4F300FD0
// http://127.0.0.1/add?hash=D3790791D1078229DFDD38DF6024E72430B91170F13333660439B2AC4F300FD0
// http://127.0.0.1/remove?hash=D3790791D1078229DFDD38DF6024E72430B91170F13333660439B2AC4F300FD0
// http://127.0.0.1/update
package hamming

import (
	"bytes"
	"fmt"

	"github.com/gonum/stat/combin"
	"github.com/steakknife/hamming"
)

// I use 64 bits words instead of bytes because I "know"
// that number of bits in my hashes is multiply of 64
// Hamming distance calculation will perform less loops
type fuzzyHash []uint64

// Sibling is placeholder keeping the closest fuzzy hash
type Sibling struct {
	s        fuzzyHash
	distance int
}

func (fh fuzzyHash) toString() string {
	var buffer bytes.Buffer
	for _, v := range fh {
		buffer.WriteString(fmt.Sprintf("%08x", v))
	}
	return buffer.String()
}

func (fh fuzzyHash) isEqual(other fuzzyHash) bool {
	if len(fh) != len(other) {
		return false
	}
	for i, e := range fh {
		if e != other[i] {
			return false
		}
	}
	return true
}

// H structure keeps hash tables for fast hamming distance calculation
// I am running lock free. Only one thread handles lookup/add/remove
// operations
// See "Fast and compact Hamming distance index" (Simon Gog, Rossano Venturini)
type H struct {
	// An array of all hashes
	hashes []fuzzyHash

	// multi index tables which store index of hashes
	multiIndexTables map[uint16]int32

	// A map of all entries in the array 'hashes'
	// I need the map for quick removal of hashes
	candidatesLookup map[string]int

	maxDistance           int     // maximum hamming distance I care of
	hashSize              int     // bits
	blocks                int     // number of blocks in the hash
	blockSize             int     // size of the block
	lastBlockSize         int     // size of the last block, often != blockSize
	lastBlockCombinations [][]int // The algorithm requires to check all 'blockSize' combinations
}

// New creates an instance of hammer distance calculator
func New(hashSize int, maxDistance int) (*H, bool) {
	if hashSize%64 != 0 {
		return &H{}, false
	}

	blocks := maxDistance + 1      // If maxDsitance is 35 bits I need 36 blocks
	blockSize := hashSize / blocks // and block size 7.11(1) bits
	lastBlockSize := blockSize     // 35 seven bits blocks and one 11 bits block
	if blocks*blockSize < hashSize {
		lastBlockSize = hashSize - ((blocks - 1) * blockSize)
	}
	// All combinations of 7 bits from 11 bits or 330 combinations
	// See https://www.wolframalpha.com/input/?i=C(11,7)
	lastBlockCombinations := combin.Combinations(lastBlockSize, blockSize)

	h := H{
		maxDistance:           maxDistance,
		hashSize:              hashSize,
		blockSize:             blockSize,
		lastBlockSize:         lastBlockSize,
		blocks:                blocks,
		lastBlockCombinations: lastBlockCombinations,
	}

	return &h, true
}

/*
Use of the table instead of the conditiosn below shaves 10% off the
performance
	d = -1
	asciicode := int(c)
	if (asciicode >= int('0')) && (asciicode <= int('9')) {
		d = asciicode - int('0')
	} else if (asciicode >= int('a')) && (asciicode <= int('f')) {
		d = 10 + asciicode - int('a')
	} else if (asciicode >= int('A')) && (asciicode <= int('F')) {
		d = 10 + asciicode - int('A')
	}
*/
var asciiCodes = []int{
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, // 0x30-0x39 48-57 0-9
	-1, -1, -1, -1, -1, -1, -1,
	10, 11, 12, 13, 14, 15, // 0x41-0x46, 65-7-1, A-F
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1,
	10, 11, 12, 13, 14, 15, // 0x61-0x66, 97-102, a-f
	-1, -1, -1, -1, -1, -1, -1, -1, // 110
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, // 260
}

// From "112233445566778899AA112233445566" to [0x1122334455667788, 0x99AA112233445566]
func hashStringToFuzzyHash(s string) (fuzzyHash, error) {
	fuzzyHash := []uint64{}
	if len(s)%2 != 0 {
		return fuzzyHash, fmt.Errorf("Bad length %d in '%s", len(s), s)
	}
	var val uint64
	bytes := 0

	for i := 0; i < len(s); i += 2 { // I reduce number of loops by processing
		d0 := asciiCodes[byte(s[i])] // two charactes (a byte) at time - 25% improvement
		if d0 < 0 {
			return fuzzyHash, fmt.Errorf("Bad character '%v' offser %d in '%s'", s[i], i, s)
		}
		d1 := asciiCodes[byte(s[i+1])]
		if d1 < 0 {
			return fuzzyHash, fmt.Errorf("Bad character '%v' offser %d in '%s'", s[i+1], i+1, s)
		}
		val = (val << 4) | uint64(d0)
		val = (val << 4) | uint64(d1)
		bytes++
		if bytes == 8 {
			fuzzyHash = append(fuzzyHash, val)
			val = 0
			bytes = 0
		}
	}
	return fuzzyHash, nil
}

func closestSibling(s []uint64, candidates []fuzzyHash) Sibling {
	sibling := Sibling{
		distance: 64 * len(s),
	}
	for _, candidate := range candidates {
		hammingDistance := hamming.Uint64s(s, candidate)
		if hammingDistance < sibling.distance {
			sibling = Sibling{
				s:        candidate,
				distance: hammingDistance,
			}
		}
	}
	return sibling
}

func (h *H) add(hash fuzzyHash) {
}

func (h *H) remove(hash fuzzyHash) {
}

func (h *H) shortestDistance(hash fuzzyHash) Sibling {
	sibling := Sibling{
		distance: h.hashSize,
	}
	return sibling
}
