package rle

import (
	"bytes"
	"encoding/binary"
	"io"
)

// Uint64Decoder is what it sounds like.
type Uint64Decoder struct {
	Value uint64
	Run   uint64
	buf   *bytes.Buffer
	err   error
}

// NewUint64Decoder returns a uint64 decoder.
func NewUint64Decoder(buf []byte) *Uint64Decoder {
	return &Uint64Decoder{
		buf: bytes.NewBuffer(buf),
	}
}

// Next returns true if a value was scanned.
func (d *Uint64Decoder) Next() bool {
	if d.Run > 1 {
		d.Run--
		return true
	}

	num, err := binary.ReadUvarint(d.buf)
	if err == io.EOF {
		return false
	}

	if err != nil {
		d.err = err
		return false
	}

	run, err := binary.ReadUvarint(d.buf)
	if err == io.EOF {
		d.err = io.ErrUnexpectedEOF
		return false
	}

	if err != nil {
		d.err = err
		return false
	}

	d.Value = num
	d.Run = run

	return true
}

// Err returns any error which occurred during decoding.
func (d *Uint64Decoder) Err() error {
	return d.err
}

// EncodeUint64 encodes run.
func EncodeUint64(nums []uint64) []byte {
	size := len(nums)

	if size == 0 {
		return nil
	}

	var b = make([]byte, 10)
	var buf bytes.Buffer
	var cur = nums[0]
	var run uint64

	for i := 0; i < size; i++ {
		num := nums[i]

		if num != cur {
			n := binary.PutUvarint(b, cur)
			buf.Write(b[:n])
			n = binary.PutUvarint(b, run)
			buf.Write(b[:n])
			cur = num
			run = 0
		}

		run++
	}

	n := binary.PutUvarint(b, cur)
	buf.Write(b[:n])
	n = binary.PutUvarint(b, run)
	buf.Write(b[:n])

	return buf.Bytes()
}

// DecodeUint64 decodes encoded run.
func DecodeUint64(buf []byte) (v []uint64, err error) {
	s := NewUint64Decoder(buf)

	for s.Next() {
		v = append(v, s.Value)
	}

	return v, s.Err()
}

// DecodeUint64Card returns a map of value cardinality.
func DecodeUint64Card(buf []byte) (v map[uint64]uint64, err error) {
	d := NewUint64Decoder(buf)
	v = make(map[uint64]uint64)

	for d.Next() {
		v[d.Value]++
	}

	return v, d.Err()
}
