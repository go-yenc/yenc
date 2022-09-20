package yenc

import (
	"fmt"
	"hash"
	"hash/crc32"
	"io"

	"gopkg.in/option.v0"
)

type Encoder struct {
	h                        Header
	w                        io.Writer
	lineOffset               int
	sizeEncoded              int
	partSize                 int
	hash                     hash.Hash32
	eol                      string
	criticalChars            []byte
	useTrailerPart           bool
	useTrailerTotal          bool
	useSinglePartAsMultiPart bool
	usePcrc32ForLastPart     bool
	useCrc32ForLastPart      bool
}

func Encode(w io.Writer, fileName string, fileSize uint64, options ...EncodeOption) (e *Encoder, err error) {
	e = option.New(options,
		EncodeWithLineMax(LineLimit),
		EncodeWithCriticalChars(DefaultCriticalChars))
	e.w = w
	e.h.Name = fileName
	e.h.Size = fileSize
	e.hash = crc32.NewIEEE()
	e.partSize = int(fileSize)
	if e.h.End > 0 {
		e.partSize = int(e.h.End - e.h.Begin)
	}
	if e.h.Total == 0 && e.useSinglePartAsMultiPart {
		e.h.Part = 1
		e.h.Total = 1
	}
	err = e.writeHeader()
	return
}

func (e *Encoder) writeHeader() (err error) {
	if e.h.Part > 0 {
		_, err = fmt.Fprintf(e.w,
			"=ybegin part=%d total=%d line=%d size=%d name=%s%s"+
				"=ypart begin=%d end=%d%s",
			e.h.Part, e.h.Total, e.h.Line, e.h.Size, e.h.Name, e.eol,
			e.h.Begin+1, e.h.End, e.eol)
	} else {
		_, err = fmt.Fprintf(e.w,
			"=ybegin line=%d size=%d name=%s%s",
			e.h.Line, e.h.Size, e.h.Name, e.eol)
	}
	if err != nil {
		err = fmt.Errorf("[yEnc] failed to write header: %w", err)
		return
	}
	return
}

func (e *Encoder) Write(b []byte) (n int, err error) {
	var (
		i, j            int
		c               byte
		atEOL, atEscape bool
	)
	_, _ = e.hash.Write(b)
	for i, j = 0, 0; i < len(b) && j < len(b); j++ {
		b[j] += 42
		c = b[j]
		atEOL = j > 0 && (e.lineOffset+j-i)%int(e.h.Line) == 0
		atEscape = e.isCriticalChar(c)
		if (atEOL || atEscape) && j-i > 0 {
			_, err = e.w.Write(b[i:j])
			n += j - i
			e.lineOffset = (e.lineOffset + j - i) % int(e.h.Line)
			e.sizeEncoded += j - i
			if err != nil {
				return
			}
			i = j
		}
		if atEOL {
			if _, err = e.w.Write([]byte(e.eol)); err != nil {
				return
			}
		}
		if atEscape {
			c += 64
			if e.lineOffset+2 > int(e.h.Line) {
				if _, err = e.w.Write([]byte(e.eol)); err != nil {
					return
				}
			}
			_, err = e.w.Write([]byte{'=', c})
			i++
			n++
			e.lineOffset = (e.lineOffset + 2) % int(e.h.Line)
			e.sizeEncoded++
			if err != nil {
				return
			}
		}
	}
	if j-i > 0 {
		_, err = e.w.Write(b[i:j])
		n += j - i
		e.lineOffset = (e.lineOffset + j - i) % int(e.h.Line)
		e.sizeEncoded += j - i
		if err != nil {
			return
		}
	}
	return
}

// CRC32 checksum of the preceeding data encoded so far.
func (e *Encoder) CRC32() uint32 {
	return e.hash.Sum32()
}

func (e *Encoder) Header() *Header {
	return &e.h
}

func (e *Encoder) Close() (err error) {
	crc32 := e.hash.Sum32()
	if _, err = fmt.Fprintf(e.w, "%s=yend size=%d", e.eol, e.sizeEncoded); err != nil {
		return
	}
	if e.useTrailerPart && e.h.Part > 0 {
		if _, err = fmt.Fprintf(e.w, " part=%d", e.h.Part); err == nil {
			return
		}
	}
	if e.useTrailerTotal && e.h.Part > 0 {
		if _, err = fmt.Fprintf(e.w, " total=%d", e.h.Total); err == nil {
			return
		}
	}
	if e.h.Part < e.h.Total || (e.usePcrc32ForLastPart && e.h.Part == e.h.Total) {
		if _, err = fmt.Fprintf(e.w, " pcrc32=%08x", crc32); err != nil {
			return
		}
	}
	if (e.h.Part == 0 && e.h.Total == 0) || (e.useCrc32ForLastPart && e.h.Part == e.h.Total) {
		if _, err = fmt.Fprintf(e.w, " crc32=%08x", crc32); err != nil {
			return
		}
	}
	if _, err = e.w.Write([]byte(e.eol)); err != nil {
		return
	}
	if e.sizeEncoded != e.partSize {
		err = fmt.Errorf("[yEnc] encode header has part size %d but actually encoded %d bytes",
			e.partSize, e.sizeEncoded)
	}
	return
}

func (e Encoder) isCriticalChar(c byte) bool {
	for i := 0; i < len(e.criticalChars); i++ {
		if c == e.criticalChars[i] {
			return true
		}
	}
	return false
}

// Specified in yEnc 1.3 as only these four characters need to be escaped for yEnc to decode the encoded stream. However
// this assumes an underlying textproto.DotWriter is used as the output Writer to encode other spcecial characters like
// dot at the start of a line.
var DefaultCriticalChars = []byte{0, '\n', '\r', '='}

// If writting directly to NNTP transport layer without a textproto.DotWriter, escaping of tab and dot characters are
// also needed. This behavior is seen in programs like yenc32.
var ExtendedCriticalChars = []byte{0, '\n', '\r', '=', '\t', '.'}

type EncodeOption func(*Encoder)

func EncodeWithPart(part uint64, total uint64, begin uint64, end uint64) EncodeOption {
	return func(e *Encoder) {
		e.h.Part = part
		e.h.Total = total
		e.h.Begin = begin
		e.h.End = end
	}
}

func EncodeWithLineMax(lineMax uint64) EncodeOption {
	return func(e *Encoder) {
		e.h.Line = lineMax
	}
}

func EncodeWithEOL(eol string) EncodeOption {
	return func(e *Encoder) {
		e.eol = eol
	}
}

// Depending on the output Writer, you may or maynot want to use CRLF or just LF for line ending. Some Writer
// implementations, like Golang's net/textproto DotWriter can handle both and normalize output to use the CRLF line
// ending. If writing directly to the transport layer, you might want to use CRLF line ending.
func EncodeWithLF() EncodeOption {
	return func(e *Encoder) {
		e.eol = "\n"
	}
}

func EncodeWithCriticalChars(chars []byte) EncodeOption {
	return func(e *Encoder) {
		e.criticalChars = chars
	}
}

func EncodeWithTrailerPart() EncodeOption {
	return func(e *Encoder) {
		e.useTrailerPart = true
	}
}

func EncodeWithTrailerTotal() EncodeOption {
	return func(e *Encoder) {
		e.useTrailerTotal = true
	}
}

// Treat a single-part file as multi-part. That is, output part=1 and total=1 keywords, and include part CRC32.
func EncodeWithSinglePartAsMultiPart() EncodeOption {
	return func(e *Encoder) {
		e.useSinglePartAsMultiPart = true
	}
}

// By default, the part CRC32 is only included if it's encoding the non-last part of a multi-part file. By applying this
// option, the part CRC32 is also included for the last part of a multi-part file.
func EncodeWithPartCrc32ForLastPart() EncodeOption {
	return func(e *Encoder) {
		e.usePcrc32ForLastPart = true
	}
}

// By default, the file CRC32 is only included if it's encoding a single-part file. By applying this option, the file
// CRC32 is also included for the last part of a multi-part file.
func EncodeWithFileCRC32ForLastPart() EncodeOption {
	return func(e *Encoder) {
		e.useCrc32ForLastPart = true
	}
}
