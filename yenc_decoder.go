package yenc

import (
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"strconv"
	"strings"

	"gopkg.in/option.v0"
	"gopkg.in/ringbuffer.v0"
)

type Decoder struct {
	h    Header
	r    io.Reader
	b    *ringbuffer.Buffer
	hash hash.Hash32
	s    int // state

	// If =ybegin keywork is not at the beginning of the data stream, returns ErrRejectPrefixData
	allowPrefixData bool
	sizeDecoded     uint64
}

func Decode(r io.Reader, options ...DecodeOption) (decoder *Decoder, err error) {
	d := option.New(options)
	if d.b == nil {
		DecodeWithBufferSize(BufferLimit)(d)
	}
	d.r = r
	d.hash = crc32.NewIEEE()
	if err = d.readHeader(); err != nil {
		return
	}
	decoder = d
	return
}

func (d *Decoder) Read(b []byte) (n int, err error) {
	var (
		i               int
		c               byte
		hasEnd, atDelim bool
	)
	for n < len(b) {
		if d.b.IsEmpty() {
			if err = d.readMore(); (err != nil && err != io.EOF) || (err == io.EOF && n == 0) {
				break
			} else if err != nil {
				err = nil
			}
		}
		if i = d.b.IndexByteFunc(matchNotCRLF); i > 0 {
			d.b.Consume(i)
			d.s = sBegin
		}
		if d.s == sBegin && d.b.HasPrefix(yend) {
			hasEnd = true
			break
		}
		if d.s == sBegin || d.s == sData {
			i, atDelim, err = d.b.ReadUntilFunc(b[n:], matchEQCRLF)
			if atDelim {
				i--
				c = b[n+i]
				if c == '=' {
					d.s = sEscape
				} else if matchCRLF(c) {
					d.s = sBegin
				}
			}
			n += i
			if err == io.EOF {
				// buffer drained out before hitting delimieter or filling output slice, should continue the line and read more
				err = nil
				continue
			}
		} else if d.s == sEscape {
			b[n] = d.b.CharAt(0) - 64
			d.b.Consume(1)
			n++
			d.s = sData
		}
	}
	if n > 0 {
		for i = 0; i < n; i++ {
			b[i] -= 42
		}
		d.sizeDecoded += uint64(n)
		d.hash.Write(b[:n])
	}
	if hasEnd {
		if err = d.consumeEnd(); err != nil {
			return
		}
	}
	if n == 0 {
		err = io.EOF
	}
	return
}

func (d *Decoder) readMore() (err error) {
	if !d.b.IsFull() {
		if _, err = d.b.ReadFrom(d.r); err == io.EOF && !d.b.IsEmpty() {
			err = nil
		}
	}
	return
}

func (d *Decoder) readHeader() (err error) {
	var (
		i                       int
		key, value              string
		hasSize, hasPart, atEOL bool
	)
	for {
		if err = d.readMore(); err != nil {
			return
		}
		if d.s == sStart {
			if !d.b.HasPrefix(ybegin) {
				// there are data before the =ybegin keyword
				if !d.allowPrefixData {
					err = ErrRejectPrefixData
					return
				}
				i = d.b.IndexByteFunc(matchCRLF)
				if i < 0 {
					d.b.Reset()
				} else {
					d.b.Consume(i + 1)
				}
				continue
			}
			d.b.Consume(len(ybegin))
			for !atEOL {
				if key, value, atEOL, err = d.readArgument(func(key string) bool { return key == "name" }); err != nil {
					return
				}
				switch key {
				case "line":
					if d.h.Line, err = strconv.ParseUint(value, 10, 64); err != nil {
						err = fmt.Errorf("[yEnc] invalid line value %#v: %w", value, ErrInvalidFormat)
						return
					}
					// We should be able to handle arbitrary line size using ring buffer. Disable this check for now.

					// Each escape uses 2 bytes, and each line includes the LF byte, so at max a line can be
					// (d.h.Line * 2) + 1 bytes.

					// lineMax := (d.h.Line * 2) + 1
					// if lineMax > uint64(d.bufferSize) {
					// 	err = fmt.Errorf("[yEnc] average line length is %d, expecting buffer requirement of %d bytes, but buffer has size %d: %w", d.h.Line, lineMax, d.bufferSize, ErrBufferTooSmall)
					// 	return
					// }
				case "size":
					if d.h.Size, err = strconv.ParseUint(value, 10, 64); err != nil {
						err = fmt.Errorf("[yEnc] invalid size value %#v: %w", value, ErrInvalidFormat)
						return
					}
					hasSize = true
				case "part":
					if d.h.Part, err = strconv.ParseUint(value, 10, 64); err != nil {
						err = fmt.Errorf("[yEnc] invalid part value %#v: %w", value, ErrInvalidFormat)
						return
					}
				case "total":
					if d.h.Total, err = strconv.ParseUint(value, 10, 64); err != nil {
						err = fmt.Errorf("[yEnc] invalid total value %#v: %w", value, ErrInvalidFormat)
						return
					}
				case "name":
					// (1.2): Leading and trailing spaces will be cut by decoders!
					d.h.Name = strings.TrimSpace(value)
					if d.h.Name == "" {
						err = fmt.Errorf("[yEnc] empty name value: %w", ErrInvalidFormat)
						return
					}
				}
			}
			if d.h.Line == 0 {
				err = fmt.Errorf("[yEnc] missing line value: %w", ErrInvalidFormat)
				return
			}
			if !hasSize {
				err = fmt.Errorf("[yEnc] missing size value: %w", ErrInvalidFormat)
				return
			}
			d.s = sBegin
			hasSize = false
		} else if d.s == sBegin {
			if i := d.b.IndexByteFunc(matchNotCRLF); i > 0 {
				d.b.Consume(i)
			}
			if d.b.HasPrefix(ypart) {
				// multipart detected
				if err = d.consumePart(); err != nil {
					return
				}
				hasPart = true
			} else if d.b.HasPrefix(yend) {
				// empty file detected, end now
				if err = d.consumeEnd(); err != nil {
					return
				}
			}
			break
		}
	}
	if d.h.Part > 1 || d.h.Total > 1 {
		// multipart checks
		if !hasPart {
			err = fmt.Errorf("[yEnc] missing =ypart line for multipart: %w", ErrInvalidFormat)
			return
		}
	}
	return
}

// =ypart keyword line is seen, now consume it.
func (d *Decoder) consumePart() (err error) {
	var (
		key, value string
		atEOL      bool
	)
	d.b.Consume(len(ypart))
	for !atEOL {
		if key, value, atEOL, err = d.readArgument(nil); err != nil {
			return
		}
		if key == "begin" {
			if d.h.Begin, err = strconv.ParseUint(value, 10, 64); err != nil {
				err = fmt.Errorf("[yEnc] invalid part begin value %#v: %w", value, ErrInvalidFormat)
				return
			}
			if d.h.Begin < 1 {
				err = fmt.Errorf("[yEnc] part begin raw value should start from 1 but got %d: %w", d.h.Begin, ErrInvalidFormat)
				return
			}
		} else if key == "end" {
			if d.h.End, err = strconv.ParseUint(value, 10, 64); err != nil {
				err = fmt.Errorf("[yEnc] invalid part end value %#v: %w", value, ErrInvalidFormat)
				return
			}
		}
	}
	if d.h.Begin == 0 {
		err = fmt.Errorf("[yEnc] no part begin value: %w", ErrInvalidFormat)
		return
	}
	d.h.Begin-- // our contract is keep Begin a 0-based index
	if d.h.End < d.h.Begin {
		err = fmt.Errorf("[yEnc] part start %d end %d: %w", d.h.Begin, d.h.End, ErrInvalidFormat)
		return
	}
	if d.h.End > d.h.Size {
		err = fmt.Errorf("[yEnc] part end %d exceeds file size %d: %w", d.h.End, d.h.Size, ErrDataCorruption)
		return
	}
	return
}

// =yend keyword line is seen, now consume it.
func (d *Decoder) consumeEnd() (err error) {
	var (
		crc32          uint32
		u64, size      uint64
		key, value     string
		hasSize, atEOL bool
	)
	crc32 = d.hash.Sum32()
	d.b.Consume(len(yend))
	for !atEOL {
		if key, value, atEOL, err = d.readArgument(nil); err != nil {
			return
		}
		if key == "size" {
			if u64, err = strconv.ParseUint(value, 10, 64); err != nil {
				err = fmt.Errorf("[yEnc] invalid trailer size value %#v: %w", value, ErrInvalidFormat)
				return
			}
			if d.h.Part > 0 {
				size = d.h.End - d.h.Begin
			} else {
				size = d.h.Size
			}
			if u64 != size {
				err = fmt.Errorf("[yEnc] header size %d != trailer size %d: %w", d.h.Size, u64, ErrDataCorruption)
				return
			}
			if d.sizeDecoded != u64 {
				err = fmt.Errorf("[yEnc] metadata has size %d but decoded data has size %d: %w", u64, d.sizeDecoded, ErrDataCorruption)
				return
			}
			hasSize = true
		} else if key == "part" {
			if u64, err = strconv.ParseUint(value, 10, 64); err != nil {
				err = fmt.Errorf("[yEnc] invalid trailer part value %#v: %w", value, ErrInvalidFormat)
				return
			}
			if u64 != d.h.Part {
				err = fmt.Errorf("[yEnc] header part %d != trailer part %d: %w", d.h.Part, u64, ErrDataCorruption)
				return
			}
		} else if key == "total" {
			if u64, err = strconv.ParseUint(value, 10, 64); err != nil {
				err = fmt.Errorf("[yEnc] invalid trailer total value %#v: %w", value, ErrInvalidFormat)
				return
			}
			if u64 != d.h.Total {
				err = fmt.Errorf("[yEnc] header total %d != trailer total %d: %w", d.h.Total, u64, ErrDataCorruption)
				return
			}
		} else if key == "pcrc32" {
			if u64, err = strconv.ParseUint(value, 16, 32); err != nil {
				err = fmt.Errorf("[yEnc] invalid trailer pcrc32 value %#v: %w", value, ErrInvalidFormat)
				return
			}
			if uint32(u64) != crc32 {
				err = fmt.Errorf("[yEnc] expect preceeding data to have CRC32 value %#08x but got %#x: %w", uint32(u64), value, ErrInvalidFormat)
				return
			}
		} else if key == "crc32" {
			if u64, err = strconv.ParseUint(value, 16, 32); err != nil {
				err = fmt.Errorf("[yEnc] invalid trailer u64 value %#v: %w", value, ErrInvalidFormat)
				return
			}
			if d.sizeDecoded == d.h.Size {
				// this is the last part, validate the final CRC32 value
				if uint32(u64) != crc32 {
					err = fmt.Errorf("[yEnc] expect final file to have CRC32 value %#08x but got %#x: %w", uint32(u64), value, ErrInvalidFormat)
					return
				}
			}
		}
	}
	if !hasSize {
		err = fmt.Errorf("[yEnc] no trailer size value: %w", ErrInvalidFormat)
		return
	}
	return
}

// Read key=value pair from the line buffer. If readToEOL is nil or returns false, value ends at space or LF. If
// readToEOL returns true, value ends at LF only.
func (d *Decoder) readArgument(readToEOL func(key string) bool) (key, value string, atEOL bool, err error) {
	var token []byte
	if i := d.b.IndexByteFunc(matchNotEQCRLF); i > 0 {
		d.b.Consume(i)
	}
	if token, err = d.b.ReadBytesFunc(matchEQCRLF); err != nil {
		err = fmt.Errorf("[yEnc] invalid keyword argument %#v: %w", token, ErrInvalidFormat)
		return
	}
	key = string(token[:len(token)-1])
	d.b.Consume(len(token))
	if readToEOL != nil && readToEOL(key) {
		if token, err = d.b.ReadBytesFunc(matchCRLF); err != nil {
			err = fmt.Errorf("[yEnc] arg value too long: %w", ErrInvalidFormat)
			return
		}
	} else {
		if token, err = d.b.ReadBytesFunc(matchSPCRLF); err != nil {
			err = fmt.Errorf("[yEnc] arg value too long: %w", ErrInvalidFormat)
			return
		}
	}
	value = string(token[:len(token)-1])
	d.b.Consume(len(token))
	if matchCRLF(token[len(token)-1]) {
		atEOL = true
	}
	return
}

// CRC32 checksum of the preceeding data decoded so far.
func (d *Decoder) CRC32() uint32 {
	return d.hash.Sum32()
}

func (d *Decoder) Header() *Header {
	return &d.h
}

// Get the remaining bytes in the buffer consumed but not decoded.
func (d *Decoder) Buffer() []byte {
	return d.b.Bytes()
}

const (
	sStart = iota
	sBegin
	sEscape
	sData
)

var ybegin = []byte("=ybegin ")
var ypart = []byte("=ypart ")
var yend = []byte("=yend ")

func matchCRLF(c byte) bool {
	return c == '\r' || c == '\n'
}

func matchEQCRLF(c byte) bool {
	return c == '=' || c == '\r' || c == '\n'
}

func matchSPCRLF(c byte) bool {
	return c == ' ' || c == '\r' || c == '\n'
}

func matchNotCRLF(c byte) bool {
	return c != '\r' && c != '\n'
}

func matchNotEQCRLF(c byte) bool {
	return c != '=' && c != '\r' && c != '\n'
}

type DecodeOption func(*Decoder)

func DecodeWithPrefixData() DecodeOption {
	return func(d *Decoder) {
		d.allowPrefixData = true
	}
}

func DecodeWithBuffer(b []byte) DecodeOption {
	return func(d *Decoder) {
		d.b = ringbuffer.New(ringbuffer.WithBuffer(b))
	}
}

func DecodeWithBufferSize(size int) DecodeOption {
	return func(d *Decoder) {
		d.b = ringbuffer.New(ringbuffer.WithSize(size))
	}
}
