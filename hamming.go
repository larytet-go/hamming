// Package hamming implelements multi-index minimal hammign distance algorithm
// See "Fast and compact Hamming distance index" (Simon Gog, Rossano Venturini)
// http://pages.di.unipi.it/rossano/wp-content/uploads/sites/7/2016/05/sigir16b.pdf
// The add/remove/dup API  is not reentrant.
// APIs add/remove modify the hash tables
// APIs dup/distance only read the hash tables
// Usually the applciation will
// duplicate the H(amming) object and switch the pointer to the instance
//
//      var currentH *hamming.H       ; all threads use this instance
//      {
//       newH := currentH.Dup()       ; clone the hash tables
//       newH.AddBulk(allMyNewHashes)
//       currentH = newH              ; Let's switch global pointer to the Hamming object
//      }
package hamming

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"unsafe"
	// For Combinations: go get -u -t gonum.org/v1/gonum/...
	// "gonum.org/v1/gonum/stat/combin"
)

// Statistics keeps all global debug counters and performance
// monitors
type Statistics struct {
	pendingDistance         uint64 // intentionally not atomic
	distance                uint64
	distanceContains        uint64
	distanceCandidate       uint64
	distanceBetterCandidate uint64
	distanceNoIndex         uint64
	distanceNoCandidates    uint64
	distanceAlreadyChecked  uint64

	addIndex        uint64
	addIndexExists  uint64
	addIndexExists1 uint64

	removeIndex          uint64
	removeIndexNotFound  uint64
	removeIndexNotFound1 uint64
	removeIndexNotFound2 uint64
	removeIndexNotFound3 uint64
}

var statistics = &Statistics{}

// FuzzyHash uses 64 bits words instead of bytes because I "know"
// that number of bits in my hashes is multiply of 64
// Hamming distance calculation will perform less loops
type FuzzyHash []uint64

// Sibling is placeholder keeping the closest fuzzy hash
type Sibling struct {
	s        FuzzyHash
	distance int
}

// ToString turns []FuzzyHash{0x00} into "0000000000000000"
func (fh FuzzyHash) ToString() string {
	var buffer bytes.Buffer
	for _, v := range fh {
		buffer.WriteString(fmt.Sprintf("%016x", v))
	}
	return buffer.String()
}

/*
I need a hashable key for the maps
Read https://github.com/golang/go/issues/25484
The naive code takes ~170ns per hash
	var buffer bytes.Buffer
	for _, v := range fh {
		for i := 0; i < 8; i++ {
			b := byte(v & 0xFF)
			buffer.WriteByte(b)
			v = v >> 8
		}
	}
	return buffer.String()

I make it under 0.5ns using 'unsafe'
*/
func (fh FuzzyHash) toKey() (s string) {
	sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&fh))
	stringHeader := (*reflect.StringHeader)(unsafe.Pointer(&s))

	stringHeader.Data = sliceHeader.Data
	stringHeader.Len = 8 * sliceHeader.Len
	// TBD I keep the hash somewhere already. Probably I do not need
	// this call to the runtime
	// runtime.KeepAlive(&fh)
	return s
}

// IsEqual compares two hashes
func (fh FuzzyHash) IsEqual(other FuzzyHash) bool {
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

func (fh FuzzyHash) and(mask uint64) uint64 {
	last := len(fh) - 1
	return fh[last] & mask
}

// Copy&paste of https://github.com/golang/go/blob/master/src/math/big/arith.go
func (fh FuzzyHash) rsh(s uint64) {
	const _W uint64 = 64
	if s == 0 {
		return
	}
	if len(fh) == 0 {
		return
	}
	s &= _W - uint64(1) // hint to the compiler that shifts by s don't need guard code
	ŝ := _W - s
	ŝ &= _W - uint64(1)                // ditto
	for i := len(fh) - 1; i > 0; i-- { // least significant item is the last in the array
		fh[i] = fh[i]>>s | fh[i-1]<<ŝ
	}
	fh[0] = fh[0] >> s
	return
}

// Dup allocates a new hash and copies the data
func (fh FuzzyHash) Dup() FuzzyHash {
	tmp := make([]uint64, len(fh))
	copy(tmp, fh)
	return tmp
}

// index table keeping sorted list of (indexes of) hashes
// key in the table is a value of block (bit substring)
// block is up to 16 bits long
type indexTable map[uint16]([]uint32)

// H structure keeps hash tables for fast hamming distance calculation
// I am running lock free. Only one thread handles lookup/add/remove
// operations
// See "Fast and compact Hamming distance index" (Simon Gog, Rossano Venturini)
type H struct {
	// An array of all hashes
	hashes []FuzzyHash

	// multi index tables storing index tables by bit substring position in the
	// hash; I support at most 256 blocks
	multiIndexTables []indexTable

	// A map of all entries in the array 'hashes'. I need the map for quick removal of hashes
	// Number of hashes I can keep wont excees 2^32-1. For 32 bytes hashes 2^32 is 140GB
	// For larger sets I can use address of the hash (uintptr)
	hashesLookup map[string]uint32

	maxDistance   int // maximum hamming distance I care of
	hashSize      int // bits
	blocks        int // number of blocks in the hash
	blockSize     int // size of the block
	lastBlockSize int // size of the last block, often != blockSize
}

// New creates an instance of hammer distance calculator
func New(hashSize int, maxDistance int) (*H, error) {
	if hashSize%64 != 0 {
		return &H{}, fmt.Errorf("hash size modulus 64 is not zero %d", hashSize)
	}

	blocks := maxDistance + 1 // If maxDsitance is 35 bits I need 36 blocks
	if blocks > 255 {
		return &H{}, fmt.Errorf("I do not support more than 255 blocks, got %d", blocks)
	}
	blockSize := hashSize / blocks // and block size 7.11(1) bits
	lastBlockSize := blockSize     // 35 seven bits blocks and one 11 bits block

	if blocks*blockSize < hashSize { // 36*7=252 < 256
		lastBlockSize = hashSize - ((blocks - 1) * blockSize) // 11 bits
	}
	// lastBlockCombinations := combin.Combinations(lastBlockSize, blockSize)

	h := H{
		maxDistance:   maxDistance,
		hashSize:      hashSize,
		blockSize:     blockSize,
		lastBlockSize: lastBlockSize,
		blocks:        blocks,

		multiIndexTables: make([]indexTable, 256),
		hashesLookup:     make(map[string]uint32),
	}

	return &h, nil
}

/*
Use of the table instead of the conditions below shaves 10% off the
CPU cycles spent
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

// HashStringToFuzzyHash converts
// "112233445566778899AA112233445566" to [FuzzyHash]{0x1122334455667788, 0x99AA112233445566}
func HashStringToFuzzyHash(s string) (FuzzyHash, error) {
	FuzzyHash := []uint64{}
	if len(s)%2 != 0 {
		return FuzzyHash, fmt.Errorf("Bad length %d in '%s", len(s), s)
	}
	var val uint64
	bytes := 0

	for i := 0; i < len(s); i += 2 { // I reduce number of loops by processing
		d0 := asciiCodes[byte(s[i])] // two charactes (a byte) at time - 25% improvement
		if d0 < 0 {
			return FuzzyHash, fmt.Errorf("Bad character '%v' offser %d in '%s'", s[i], i, s)
		}
		d1 := asciiCodes[byte(s[i+1])]
		if d1 < 0 {
			return FuzzyHash, fmt.Errorf("Bad character '%v' offser %d in '%s'", s[i+1], i+1, s)
		}
		val = (val << 4) | uint64(d0)
		val = (val << 4) | uint64(d1)
		bytes++
		if bytes == 8 {
			FuzzyHash = append(FuzzyHash, val)
			val = 0
			bytes = 0
		}
	}
	return FuzzyHash, nil
}

const (
	m1q uint64 = 0x5555555555555555
	m2q        = 0x3333333333333333
	m4q        = 0x0f0f0f0f0f0f0f0f
	hq         = 0x0101010101010101
)

// Copy from github.com/steakknife/hamming"
func distanceUint64s(b0, b1 []uint64) int {
	d := 0
	for i, b0i := range b0 {
		x := b0i ^ b1[i]
		// put count of each 2 bits into those 2 bits
		x -= (x >> 1) & m1q

		// put count of each 4 bits into those 4 bits
		x = (x & m2q) + ((x >> 2) & m2q)

		// put count of each 8 bits into those 8 bits
		x = (x + (x >> 4)) & m4q

		// returns left 8 bits of x + (x<<8) + (x<<16) + (x<<24) + ...
		d += int((x * hq) >> 56)
	}
	return d
}

func closestSibling(s []uint64, hashes []FuzzyHash) Sibling {
	sibling := Sibling{
		distance: 64 * len(s),
	}
	for _, hash := range hashes {
		hammingDistance := distanceUint64s(s, hash)
		if hammingDistance < sibling.distance {
			sibling = Sibling{
				s:        hash,
				distance: hammingDistance,
			}
		}
	}
	return sibling
}

// Recipe from https://play.golang.org/p/k53JzyvnE0
func addMultiindex(multiIndexTables []indexTable, blockIndex uint8, blockValue uint16, hashIndex uint32, preallocate int) {
	if multiIndexTables[blockIndex] == nil {
		multiIndexTables[blockIndex] = make(map[uint16]([]uint32))
	}
	indexTable := multiIndexTables[blockIndex]
	if _, ok := indexTable[blockValue]; !ok {
		indexTable[blockValue] = make([]uint32, preallocate)
	}
	hashes := indexTable[blockValue]
	insertIndex := sort.Search(len(hashes), func(i int) bool { return hashes[i] >= hashIndex })
	if (len(hashes) > insertIndex) && (hashes[insertIndex] == hashIndex) {
		statistics.addIndexExists1++
		return
	}
	hashes = append(hashes, 0)
	copy(hashes[insertIndex+1:], hashes[insertIndex:])
	hashes[insertIndex] = hashIndex
	indexTable[blockValue] = hashes
	multiIndexTables[blockIndex] = indexTable
	// fmt.Printf("blockIndex %d, blockValue %d, hashIndex %d\n", blockIndex, blockValue, hashIndex)
	// fmt.Printf("hashes[insertIndex]=%v,indexTable[blockValue]=%v,multiIndexTables[blockIndex]=%v\n",
	// 	hashes[insertIndex], indexTable[blockValue], multiIndexTables[blockIndex])
}

func removeMultiindex(multiIndexTables []indexTable, blockIndex uint8, blockValue uint16, hashIndex uint32, preallocate int) {
	if multiIndexTables[blockIndex] == nil {
		statistics.removeIndexNotFound1++
		return
	}
	indexTable := multiIndexTables[blockIndex]
	if _, ok := indexTable[blockValue]; !ok {
		statistics.removeIndexNotFound2++
		return
	}
	hashes := indexTable[blockValue]
	removeIndex := sort.Search(len(hashes), func(i int) bool { return hashes[i] >= hashIndex })
	if (len(hashes) <= removeIndex) || (hashes[removeIndex] == hashIndex) {
		statistics.removeIndexNotFound3++
		return
	}
	copy(hashes[removeIndex:], hashes[removeIndex+1:])
	hashes = hashes[:len(hashes)-1]
	indexTable[blockValue] = hashes
	multiIndexTables[blockIndex] = indexTable
}

func (h *H) add(hash FuzzyHash) bool {
	statistics.addIndex++
	key := hash.toKey()
	if _, ok := h.hashesLookup[key]; ok {
		statistics.addIndexExists++
		return false
	}
	// add the new hash to the end of the list
	hashIndex := uint32(len(h.hashes))
	h.hashes = append(h.hashes, hash)

	// I maintain a map for quick removing a hash
	h.hashesLookup[key] = uint32(hashIndex)

	// Add hashIndex to the sorted arrays in multiIndexTables
	hash = hash.Dup()
	blockMask := (uint64(1) << uint64(h.blockSize)) - 1
	preallocationSize := len(h.hashesLookup) / (1 << uint(h.blockSize)) // Roughly half of what I need
	for b := uint8(0); b < uint8(h.blocks); b++ {
		blockValue := hash.and(blockMask)
		hash.rsh(uint64(h.blockSize))
		addMultiindex(h.multiIndexTables, b, uint16(blockValue), hashIndex, preallocationSize)
	}
	// fmt.Printf("h.hashes=%v\n", h.hashes)

	// The last bock can be larger than h.blockSize
	// I want to add all Combinations(h.lastBlockSize, h.blockSize)
	// If lastBlockSize is 11 and blockSize is 7
	// C(11,7)= {{0,1,2,3,4,5,6}, {1,2,3,4,5,6,7}, ... } - 330 combinations
	//blockValues := generateBitCombinations(hash[len(hash)-1], h.lastBlockCombinations)
	//for _, blockValue := range blockValues {
	//        removeMultiindex(h.multiIndexTables, uint16(blockValue), hashIndex, preallocationSize)
	//}

	return true
}

func (h *H) remove(hash FuzzyHash) bool {
	statistics.removeIndex++
	key := hash.toKey()
	if _, ok := h.hashesLookup[key]; !ok {
		statistics.removeIndexNotFound++
		return false
	}

	// I maintain a map for quick removing a hash
	hashIndex := uint32(h.hashesLookup[key])

	// Remove hashIndex from the sorted arrays in multiIndexTables
	blockMask := (uint64(1) << uint64(h.blockSize)) - 1
	preallocationSize := len(h.hashesLookup) / (1 << uint(h.blockSize)) // Roughly half of what I need
	for b := uint8(0); b < uint8(h.blocks); b++ {
		blockValue := hash.and(blockMask)
		hash.rsh(uint64(h.blockSize))
		removeMultiindex(h.multiIndexTables, b, uint16(blockValue), hashIndex, preallocationSize)
	}

	return true
}

func generateBitCombinations(value uint64, combinations [][]int) (r []uint64) {
	for _, c := range combinations {
		blockValue := uint64(0)
		for bitDst, bitSrc := range c {
			bitValue := (value & (uint64(1) << uint(bitSrc))) >> uint(bitSrc)
			blockValue |= bitValue << uint(bitDst)
		}
		r = append(r, blockValue)
	}
	return r
}

// AddBulk adds specified hashes to the DB
// This API is not reentrant and should not be called simultaneously
// with add/remove/dup/distance
func (h *H) AddBulk(hashes []FuzzyHash) bool {
	ok := true
	for _, hash := range hashes {
		ok = ok && h.add(hash)
	}
	return ok
}

// RemoveBulk removes specified hashes from the DB
// This API is not reentrant and should not be called simultaneously
// with add/remove/dup/distance
func (h *H) RemoveBulk(hashes []FuzzyHash) bool {
	ok := true
	for _, hash := range hashes {
		ok = ok && h.remove(hash)
	}
	return ok
}

// RemoveAll clears the DB
// This API is not reentrant and should not be called simultaneously
// with add/remove/dup/distance
func (h *H) RemoveAll() {
	h.multiIndexTables = make([]indexTable, 256)
	h.hashesLookup = make(map[string]uint32)
}

// Contains returns true if the hash is in the DB
// This API is not reentrant and should not be called simultaneously
// with add/remove
func (h *H) Contains(hash FuzzyHash) bool {
	key := hash.toKey()
	_, ok := h.hashesLookup[key]
	return ok
}

// ShortestDistance returns the closest sibling in the DB for
// the specfied hash
// This API is not reentrant and should not be called simultaneously
// with add/remove
func (h *H) ShortestDistance(hash FuzzyHash) Sibling {
	statistics.distance++
	statistics.pendingDistance++
	defer func() {
		statistics.pendingDistance--
	}()

	// Do I have this hash already?
	if h.Contains(hash) {
		statistics.distanceContains++
		return Sibling{distance: 0, s: hash}
	}
	sibling := Sibling{
		distance: h.hashSize,
	}

	// for all 7 bits sub-strings in the 'hash'
	// find all hashes  containing exactly the same hash
	// Choose a sibling with the minimum hamming distance from the 'hash'
	blockMask := (uint64(1) << uint64(h.blockSize)) - 1
	hashOrig := hash
	hash = hash.Dup()
	//fmt.Printf("%v\n", h.multiIndexTables)
	//fmt.Printf("disatnce.h.hashes=%v\n", h.hashes)

	// Keeping map of already checked hashes slows the code by factor two
	// var checkedCandidates map[string](bool)
	for b := uint8(0); b < uint8(h.blocks); b++ {
		blockValue := hash.and(blockMask)
		hash.rsh(uint64(h.blockSize))
		indexTable := h.multiIndexTables[b]
		if indexTable == nil {
			statistics.distanceNoIndex++
			continue
		}
		candidates, ok := indexTable[uint16(blockValue)]
		if !ok {
			statistics.distanceNoCandidates++
			continue
		}
		statistics.distanceCandidate += uint64(len(candidates))
		for _, candidateIndex := range candidates {
			candidateHash := h.hashes[candidateIndex]
			hammingDistance := distanceUint64s(hashOrig, candidateHash)
			// fmt.Printf("Sample %s Candidate %s distance %d blockV=%x hash=%s\n",
			//	hashOrig.ToString(), candidateHash.ToString(), hammingDistance, blockValue, hash.ToString())
			if hammingDistance < sibling.distance {
				statistics.distanceBetterCandidate++
				sibling = Sibling{
					s:        candidateHash,
					distance: hammingDistance,
				}
			}
		}
	}

	return sibling
}

// Dup allocates RAM and copies the tables
// This API is not reentrant and should not be called simultaneously
// with add/remove
func (h *H) Dup() *H {
	newH, _ := New(h.hashSize, h.maxDistance)
	newH.hashes = make([]FuzzyHash, len(h.hashes))
	copy(newH.hashes, h.hashes)
	for blockIndex, indexTable := range h.multiIndexTables {
		tmpIndexTable := make(map[uint16]([]uint32))
		newH.multiIndexTables[blockIndex] = tmpIndexTable
		for blockValue, hashes := range indexTable {
			tmpIndexTable[blockValue] = make([]uint32, len(hashes))
			copy(tmpIndexTable[blockValue], indexTable[blockValue])
		}
	}
	for key, value := range h.hashesLookup {
		newH.hashesLookup[key] = value
	}
	return newH
}
