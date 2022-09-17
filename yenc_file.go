package yenc

// yEncode header information
type Header struct {
	Name   string  // Name of the final output file
	Size   uint64  // Final, overall file size (of all parts decoded)
	Part   uint64  // Part number (starts from 1)
	Total  uint64  // Total number of parts. Optional even for multipart.
	Line   uint64  // Average line length
	Begin  uint64  // Part begin offset (0-indexed). Note the begin keyword in the =ypart line is 1-indexed.
	End    uint64  // Part end offset (0-indexed, exclusive)
	Pcrc32 *uint32 // CRC32 of the preceeding encoded part upto and includes the current part. Required for multipart.
	Crc32  *uint32 // CRC32 of the entire encoded binary. Optional.
}
