package yenc

// yEncode header information
type Header struct {
	Name  string // Name of the final output file
	Size  uint64 // Final, overall file size (of all parts decoded)
	Part  uint64 // Part number (starts from 1)
	Total uint64 // Total number of parts. Optional even for multipart.
	Line  uint64 // Average line length
	Begin uint64 // Part begin offset (0-indexed). Note the begin keyword in the =ypart line is 1-indexed.
	End   uint64 // Part end offset (0-indexed, exclusive)
}

// Max number of bytes per line (ends in LF and includes the LF) when decoding yEnc data stream. Default is 4096 which
// is larger than the setting in probably all known NNTP and yEncode implementations.
var BufferLimit = 4096

// Max number of bytes per line (ends in LF but NOT include the LF) when encoding yEnc data stream. Default is 128 which
// is a commonly acceptable limit for probably all known NNTP and yEncode implementations.
var LineLimit uint64 = 128
