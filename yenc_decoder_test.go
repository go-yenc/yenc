package yenc

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestDecodeYEnc32(t *testing.T) {
	var b bytes.Buffer
	for i := 1; i <= 10; i++ {
		f, err := os.Open(fmt.Sprintf("fixture/yenc32-%03d.ntx", i))
		if err != nil {
			t.Fatal(err)
		}

		d, err := Decode(f, DecodeWithBufferSize(200))
		if err != nil {
			t.Fatal(err)
		}

		fmt.Printf("%#v\n", d.Header())

		_, err = io.Copy(&b, d)
		if err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open("fixture/yenc32-raw.bin")
	if err != nil {
		t.Fatal(err)
	}
	identical, err := Diff(f, &b)
	if err != nil {
		t.Fatal(err)
	}
	if !identical {
		t.Error("yenc32 decode output mismatch!")
	}
}

func TestDecodeNgPost(t *testing.T) {
	var b bytes.Buffer
	for i := 1; i <= 10; i++ {
		f, err := os.Open(fmt.Sprintf("fixture/ngPost-%03d.ntx", i))
		if err != nil {
			t.Fatal(err)
		}

		d, err := Decode(f, DecodeWithBufferSize(200))
		if err != nil {
			t.Fatal(err)
		}

		fmt.Printf("%#v\n", d.Header())

		_, err = io.Copy(&b, d)
		if err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open("fixture/ngPost-raw.bin")
	if err != nil {
		t.Fatal(err)
	}
	identical, err := Diff(f, &b)
	if err != nil {
		t.Fatal(err)
	}
	if !identical {
		t.Error("ngPost decode output mismatch!")
	}
}

func TestDecodeCamelsystemPowerpost(t *testing.T) {
	var b bytes.Buffer
	f, err := os.Open("fixture/CamelsystemPowerpost-001.ntx")
	if err != nil {
		t.Fatal(err)
	}

	d, err := Decode(f, DecodeWithBufferSize(200))
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%#v\n", d.Header())

	_, err = io.Copy(&b, d)
	if err != nil {
		t.Fatal(err)
	}
	f, err = os.Open("fixture/CamelsystemPowerpost-raw.bin")
	if err != nil {
		t.Fatal(err)
	}
	identical, err := Diff(f, &b)
	if err != nil {
		t.Fatal(err)
	}
	if !identical {
		t.Error("CamelsystemPowerpost decode output mismatch!")
	}
}

func TestDecodeYEncBinPoster(t *testing.T) {
	var b bytes.Buffer
	f, err := os.Open("fixture/yEncBinPoster-001.ntx")
	if err != nil {
		t.Fatal(err)
	}

	d, err := Decode(f, DecodeWithBufferSize(200))
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%#v\n", d.Header())

	_, err = io.Copy(&b, d)
	if err != nil {
		t.Fatal(err)
	}
	f, err = os.Open("fixture/yEncBinPoster-raw.bin")
	if err != nil {
		t.Fatal(err)
	}
	identical, err := Diff(f, &b)
	if err != nil {
		t.Fatal(err)
	}
	if !identical {
		t.Error("yEncBinPoster decode output mismatch!")
	}
}

func TestDecodeYencPowerPost(t *testing.T) {
	var b bytes.Buffer
	f, err := os.Open("fixture/YencPowerPost-001.ntx")
	if err != nil {
		t.Fatal(err)
	}

	d, err := Decode(f, DecodeWithBufferSize(200))
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%#v\n", d.Header())

	_, err = io.Copy(&b, d)
	if err != nil {
		t.Fatal(err)
	}
	f, err = os.Open("fixture/YencPowerPost-raw.bin")
	if err != nil {
		t.Fatal(err)
	}
	identical, err := Diff(f, &b)
	if err != nil {
		t.Fatal(err)
	}
	if !identical {
		t.Error("YencPowerPost decode output mismatch!")
	}
}

// Diff compares the contents of two io.Readers.
// The return value of identical is true if and only if there are no errors
// in reading r1 and r2 (io.EOF excluded) and r1 and r2 are
// byte-for-byte identical.
func Diff(r1, r2 io.Reader) (identical bool, err error) {
	buf1 := bufio.NewReader(r1)
	buf2 := bufio.NewReader(r2)
	for {
		const sz = 1024
		scratch1 := make([]byte, sz)
		scratch2 := make([]byte, sz)
		n1, err1 := buf1.Read(scratch1)
		n2, err2 := buf2.Read(scratch2)
		if err1 != nil && err1 != io.EOF {
			return false, err1
		}
		if err2 != nil && err2 != io.EOF {
			return false, err2
		}
		if err1 == io.EOF || err2 == io.EOF {
			return err1 == err2, nil
		}
		if !bytes.Equal(scratch1[0:n1], scratch2[0:n2]) {
			return false, nil
		}
	}
}

func TestDecodeNyuu(t *testing.T) {
	var b bytes.Buffer
	f, err := os.Open("fixture/260731a73db67e8095a5eaf0b64b9d3db0117cdb@nyuu.ntx")
	if err != nil {
		t.Fatal(err)
	}

	d, err := Decode(f, DecodeWithBufferSize(200))
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%#v\n", d.Header())

	_, err = io.Copy(&b, d)
	if err != nil {
		t.Fatal(err)
	}
	f, err = os.Open("fixture/260731a73db67e8095a5eaf0b64b9d3db0117cdb-raw.bin")
	if err != nil {
		t.Fatal(err)
	}
	identical, err := Diff(f, &b)
	if err != nil {
		t.Fatal(err)
	}
	if !identical {
		t.Error("Nyuu decode output mismatch!")
	}
}
