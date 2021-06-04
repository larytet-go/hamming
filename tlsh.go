/* Copyright 2017 Lukas Rist

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. */

package hamming

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"io"
	"math"
	"os"
)

const (
	log1_5       = 0.4054651
	log1_3       = 0.26236426
	log1_1       = 0.095310180
	codeSize     = 32
	windowLength = 5
	effBuckets   = 128
	numBuckets   = 256
)

// Tlsh holds hash components
type Tlsh struct {
	checksum byte
	lValue   byte
	q1Ratio  byte
	q2Ratio  byte
	qRatio   byte
	Code     [codeSize]byte
}

// New represents type factory for Tlsh
func NewTlsh(checksum, lValue, q1Ratio, q2Ratio, qRatio byte, code [codeSize]byte) *Tlsh {
	return &Tlsh{
		checksum: checksum,
		lValue:   lValue,
		q1Ratio:  q1Ratio,
		q2Ratio:  q2Ratio,
		qRatio:   qRatio,
		Code:     code,
	}
}

// Binary returns the binary representation of the hash
func (t *Tlsh) Binary() []byte {
	return append([]byte{swapByte(t.checksum), swapByte(t.lValue), t.qRatio}, t.Code[:]...)
}

// String returns the string representation of the hash`
func (t *Tlsh) String() string {
	return hex.EncodeToString(t.Binary())
}

// Parsing the hash of the string type 
func ParseStringToTlsh(hashString string) (*Tlsh, error) {
	var code [codeSize]byte
	hashByte, err := hex.DecodeString(hashString)
	if err != nil {
		return &Tlsh{}, err
	}
	chechsum := swapByte(hashByte[0])
	lValue := swapByte(hashByte[1])
	qRatio := hashByte[2]
	q1Ratio := (qRatio >> 4) & 0xF
	q2Ratio := qRatio & 0xF
	copy(code[:], hashByte[3:])
	return NewTlsh(chechsum, lValue, q1Ratio, q2Ratio, qRatio, code), nil
}

func quartilePoints(buckets [numBuckets]uint) (q1, q2, q3 uint) {
	var spl, spr uint
	p1 := uint(effBuckets/4 - 1)
	p2 := uint(effBuckets/2 - 1)
	p3 := uint(effBuckets - effBuckets/4 - 1)
	end := uint(effBuckets - 1)

	bucketCopy := make([]uint, effBuckets)
	copy(bucketCopy, buckets[:effBuckets])

	shortCutLeft := make([]uint, effBuckets)
	shortCutRight := make([]uint, effBuckets)

	for l, r := uint(0), end; ; {
		ret := partition(bucketCopy, l, r)
		if ret > p2 {
			r = ret - 1
			shortCutRight[spr] = ret
			spr++
		} else if ret < p2 {
			l = ret + 1
			shortCutLeft[spl] = ret
			spl++
		} else {
			q2 = bucketCopy[p2]
			break
		}
	}

	shortCutLeft[spl] = p2 - 1
	shortCutRight[spr] = p2 + 1

	for i, l := uint(0), uint(0); i <= spl; i++ {
		r := shortCutLeft[i]
		if r > p1 {
			for {
				ret := partition(bucketCopy, l, r)
				if ret > p1 {
					r = ret - 1
				} else if ret < p1 {
					l = ret + 1
				} else {
					q1 = bucketCopy[p1]
					break
				}
			}
			break
		} else if r < p1 {
			l = r
		} else {
			q1 = bucketCopy[p1]
			break
		}
	}

	for i, r := uint(0), end; i <= spr; i++ {
		l := shortCutRight[i]
		if l < p3 {
			for {
				ret := partition(bucketCopy, l, r)
				if ret > p3 {
					r = ret - 1
				} else if ret < p3 {
					l = ret + 1
				} else {
					q3 = bucketCopy[p3]
					break
				}
			}
			break
		} else if l > p3 {
			r = l
		} else {
			q3 = bucketCopy[p3]
			break
		}
	}

	return q1, q2, q3
}

func partition(buf []uint, left, right uint) uint {
	if left == right {
		return left
	}

	if left+1 == right {
		if buf[left] > buf[right] {
			buf[right], buf[left] = buf[left], buf[right]
		}
		return left
	}

	ret := left
	pivot := (left + right) >> 1
	val := buf[pivot]

	buf[pivot] = buf[right]
	buf[right] = val

	for i := left; i < right; i++ {
		if buf[i] < val {
			buf[i], buf[ret] = buf[ret], buf[i]
			ret++
		}
	}

	buf[right] = buf[ret]
	buf[ret] = val

	return ret
}

func lValue(length int) byte {
	var l byte

	if length <= 656 {
		l = byte(math.Floor(math.Log(float64(length)) / log1_5))
	} else if length <= 3199 {
		l = byte(math.Floor(math.Log(float64(length))/log1_3 - 8.72777))
	} else {
		l = byte(math.Floor(math.Log(float64(length))/log1_1 - 62.5472))
	}

	return l % 255
}

func swapByte(in byte) byte {
	var out byte

	out = ((in & 0xF0) >> 4) & 0x0F
	out |= ((in & 0x0F) << 4) & 0xF0

	return out
}

func bucketsBinaryRepresentation(buckets [numBuckets]uint, q1, q2, q3 uint) [codeSize]byte {
	var biHash [codeSize]byte

	for i := 0; i < codeSize; i++ {
		var h byte
		for j := 0; j < 4; j++ {
			k := buckets[4*i+j]
			if q3 < k {
				h += 3 << (byte(j) * 2)
			} else if q2 < k {
				h += 2 << (byte(j) * 2)
			} else if q1 < k {
				h += 1 << (byte(j) * 2)
			}
		}
		// Prepend the new h to the hash
		biHash[(codeSize-1)-i] = h
	}
	return biHash
}

func reverse(s [5]byte) [5]byte {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// This is a kind of randomly picked mapping table
var vTable = [256]byte{
	1, 87, 49, 12, 176, 178, 102, 166, 121, 193, 6, 84, 249, 230, 44, 163,
	14, 197, 213, 181, 161, 85, 218, 80, 64, 239, 24, 226, 236, 142, 38, 200,
	110, 177, 104, 103, 141, 253, 255, 50, 77, 101, 81, 18, 45, 96, 31, 222,
	25, 107, 190, 70, 86, 237, 240, 34, 72, 242, 20, 214, 244, 227, 149, 235,
	97, 234, 57, 22, 60, 250, 82, 175, 208, 5, 127, 199, 111, 62, 135, 248,
	174, 169, 211, 58, 66, 154, 106, 195, 245, 171, 17, 187, 182, 179, 0, 243,
	132, 56, 148, 75, 128, 133, 158, 100, 130, 126, 91, 13, 153, 246, 216, 219,
	119, 68, 223, 78, 83, 88, 201, 99, 122, 11, 92, 32, 136, 114, 52, 10,
	138, 30, 48, 183, 156, 35, 61, 26, 143, 74, 251, 94, 129, 162, 63, 152,
	170, 7, 115, 167, 241, 206, 3, 150, 55, 59, 151, 220, 90, 53, 23, 131,
	125, 173, 15, 238, 79, 95, 89, 16, 105, 137, 225, 224, 217, 160, 37, 123,
	118, 73, 2, 157, 46, 116, 9, 145, 134, 228, 207, 212, 202, 215, 69, 229,
	27, 188, 67, 124, 168, 252, 42, 4, 29, 108, 21, 247, 19, 205, 39, 203,
	233, 40, 186, 147, 198, 192, 155, 33, 164, 191, 98, 204, 165, 180, 117, 76,
	140, 36, 210, 172, 41, 54, 159, 8, 185, 232, 113, 196, 231, 47, 146, 120,
	51, 65, 28, 144, 254, 221, 93, 189, 194, 139, 112, 43, 71, 109, 184, 209,
}

func pearsonHash(salt byte, keys *[3]byte) (h byte) {
	h = vTable[h^salt]
	h = vTable[h^keys[0]]
	h = vTable[h^keys[1]]
	h = vTable[h^keys[2]]
	return
}

func fillBuckets(r FuzzyReader) ([numBuckets]uint, byte, int, error) {
	buckets := [numBuckets]uint{}
	chunkSlice := make([]byte, windowLength)
	chunk := [windowLength]byte{}
	salt := [6]byte{2, 3, 5, 7, 11, 13}
	fileSize := 0
	checksum := byte(0)

	n, err := r.Read(chunkSlice)
	if err != nil {
		return [numBuckets]uint{}, 0, 0, err
	}
	copy(chunk[:], chunkSlice[0:5])
	chunk = reverse(chunk)
	fileSize += n

	chunk3 := &[3]byte{}

	for {
		chunk3[0] = chunk[0]
		chunk3[1] = chunk[1]
		chunk3[2] = checksum
		checksum = pearsonHash(0, chunk3)

		chunk3[2] = chunk[2]
		buckets[pearsonHash(salt[0], chunk3)]++

		chunk3[2] = chunk[3]
		buckets[pearsonHash(salt[1], chunk3)]++

		chunk3[1] = chunk[2]
		buckets[pearsonHash(salt[2], chunk3)]++

		chunk3[2] = chunk[4]
		buckets[pearsonHash(salt[3], chunk3)]++

		chunk3[1] = chunk[1]
		buckets[pearsonHash(salt[4], chunk3)]++

		chunk3[1] = chunk[3]
		buckets[pearsonHash(salt[5], chunk3)]++

		copy(chunk[1:], chunk[0:4])
		chunk[0], err = r.ReadByte()
		if err != nil {
			if err != io.EOF {
				return [numBuckets]uint{}, 0, 0, err
			}
			break
		}
		fileSize++
	}
	return buckets, checksum, fileSize, nil
}

// hashCalculate calculate TLSH
func hashCalculate(r FuzzyReader) (*Tlsh, error) {
	buckets, checksum, fileSize, err := fillBuckets(r)
	if err != nil {
		return &Tlsh{}, err
	}

	q1, q2, q3 := quartilePoints(buckets)
	q1Ratio := byte(float32(q1)*100/float32(q3)) % 16
	q2Ratio := byte(float32(q2)*100/float32(q3)) % 16
	qRatio := ((q1Ratio & 0xF) << 4) | (q2Ratio & 0xF)

	biHash := bucketsBinaryRepresentation(buckets, q1, q2, q3)

	return NewTlsh(checksum, lValue(fileSize), q1Ratio, q2Ratio, qRatio, biHash), nil
}

// FuzzyReader interface
type FuzzyReader interface {
	io.Reader
	io.ByteReader
}

//HashReader calculates the TLSH for the input reader
func HashReader(r FuzzyReader) (tlsh *Tlsh, err error) {
	tlsh, err = hashCalculate(r)
	if err != nil {
		return &Tlsh{}, err
	}
	return tlsh, err
}

//HashBytes calculates the TLSH for the input byte slice
func HashBytes(blob []byte) (tlsh *Tlsh, err error) {
	r := bytes.NewReader(blob)
	return HashReader(r)
}

//HashFilename calculates the TLSH for the input file
func HashFilename(filename string) (tlsh *Tlsh, err error) {
	f, err := os.Open(filename)
	defer f.Close()
	if err != nil {
		return &Tlsh{}, err
	}

	r := bufio.NewReader(f)
	return HashReader(r)
}

