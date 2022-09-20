package yenc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestEncoder(t *testing.T) {
	in, err := os.Open("fixture/encode-raw.bin")
	if err != nil {
		t.Fatal(err)
	}
	stat, err := in.Stat()
	if err != nil {
		t.Fatal(err)
	}
	fileSize := uint64(stat.Size())
	partSize := uint64(512)
	total := fileSize / partSize
	if fileSize%partSize > 0 {
		total += 1
	}
	var (
		e         *Encoder
		b         bytes.Buffer
		fb        []byte
		n         int64
		out       *os.File
		identical bool
	)

	for part, begin, end := uint64(1), uint64(0), uint64(partSize); begin < fileSize; part, begin, end = part+1, begin+partSize, end+partSize {
		b.Reset()

		if end > fileSize {
			end = fileSize
		}

		if e, err = Encode(&b, stat.Name(), fileSize,
			EncodeWithPart(part, total, begin, end),
			EncodeWithLF(),
			EncodeWithPartCrc32ForLastPart()); err != nil {
			t.Fatal(err)
		}
		if n, err = io.Copy(e, &io.LimitedReader{R: in, N: int64(end - begin)}); err != nil {
			t.Fatal(err)
		}
		if uint64(n) != end-begin {
			t.Errorf("yEnc encode part %d: expect to write %d bytes but instead wrote %d bytes", part, end-begin, n)
		}
		if err = e.Close(); err != nil {
			t.Fatal(err)
		}

		if out, err = os.Open(fmt.Sprintf("fixture/encode-%03d.ntx", part)); err != nil {
			t.Fatal(err)
		}

		fb = b.Bytes()
		if identical, err = Diff(&b, out); err != nil {
			t.Fatal(err)
		}
		if !identical {
			t.Errorf("part %d encode mismatch", part)
			t.Log("\n" + hex.Dump(fb))
		}
	}
}
