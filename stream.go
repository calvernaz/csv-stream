package csv

import (
	"io"
	"strings"
	"fmt"
)

type SyntaxError struct {
	msg    string // description of error
	Offset int64  // error occurred after reading Offset bytes
}

func (e *SyntaxError) Error() string { return e.msg }

// A Decoder reads and decodes CSV values from an input stream.
type Decoder struct {
	r     io.Reader
	buf   []byte
	scanp int // start of unread data in buf
	err   error
	
	tokenState int
	tokenStack []int
}

// NewDecoder returns a new decoder that reads from r.
//
// The decoder introduces its own buffering and may
// read data from r beyond the CSV values requested.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

type Header interface{}

func (dec *Decoder) Header() (Header, error) {
	return nil, nil
}

func (dec *Decoder) Decode() ([]string, error) {
	if dec.err != nil {
		return nil, dec.err
	}
	
	if err := dec.tokenPrepareForDecode(); err != nil {
		return nil, err
	}
	
	n, err := dec.readValue()
	if err != nil {
		return nil, err
	}
	
	line := string(dec.buf[dec.scanp: dec.scanp+n])
	dec.scanp += n
	dec.scanp++
	return strings.Split(line, ","), nil
}

func (dec *Decoder) peek() (byte, error) {
	var err error
	fmt.Println("------- b:peek --------")
	
	for {
		fmt.Println("for i := dec.scanp; i < len(dec.buf); i++")
		fmt.Printf("for i := %d; i < %d; i++\n", dec.scanp, len(dec.buf))
		for i := dec.scanp; i < len(dec.buf); i++ {
			c := dec.buf[i]
			fmt.Println(">> c := dec.buf[i]")
			fmt.Printf("%v := dec.buf[%d]\n", string(c), i)
			if isSpace(c) {
				continue
			}
			dec.scanp = i
			return c, nil
		}
		// buffer has been scanned, now report any error
		if err != nil {
			fmt.Println("------- e:peek --------")
			return 0, err
		}
		err = dec.refill()
	}
}

func (dec *Decoder) refill() error {
	fmt.Println("------- b:refill --------")
	// Make room to read more into the buffer.
	// First slide down data already consumed.
	fmt.Printf("dec.scanp %d\n", dec.scanp)
	if dec.scanp > 0 {
		fmt.Printf("copy(dec.buf[%d], dec.buf[%d])\n", len(dec.buf), len(dec.buf[dec.scanp:]))
		n := copy(dec.buf, dec.buf[dec.scanp:])
		dec.buf = dec.buf[:n]
		fmt.Printf("dec.buf[:%d]\n", len(dec.buf))
		dec.scanp = 0
		fmt.Printf(">> reset: dec.scanp = 0 \n")
	}
	
	// Grow buffer if not large enough.
	const minRead = 512
	fmt.Println(">> if cap(dec.buf)-len(dec.buf) < minRead {")
	fmt.Printf("if %d - %d < %d {\n", cap(dec.buf), len(dec.buf), minRead)
	if cap(dec.buf)-len(dec.buf) < minRead {
		newBuf := make([]byte, len(dec.buf), 2*cap(dec.buf)+minRead)
		fmt.Println(">> newBuf := make([]byte, len(dec.buf), 2*cap(dec.buf)+minRead)")
		fmt.Printf("newBuf := make([]byte, %d, %d)\n",len(dec.buf), 2*cap(dec.buf)+minRead)
		copy(newBuf, dec.buf)
		fmt.Println(">> copy(newBuf, dec.buf)")
		dec.buf = newBuf
		fmt.Println(">> dec.buf = newBuf")
	}
	
	// Read. Delay error for next iteration (after scan).
	n, err := dec.r.Read(dec.buf[len(dec.buf):cap(dec.buf)])
	fmt.Printf(">> n, err := dec.r.Read(dec.buf[len(dec.buf):cap(dec.buf)])\n")
	fmt.Printf("%d, err := dec.r.Read(dec.buf[%d:%d])\n", n, len(dec.buf), cap(dec.buf))
	dec.buf = dec.buf[0: len(dec.buf)+n]
	fmt.Println(">> dec.buf = dec.buf[0: len(dec.buf)+n]")
	fmt.Printf("dec.buf = dec.buf[0: %d]\n", len(dec.buf)+n)
	
	fmt.Println("------- e:refill --------")
	return err
}

// More reports whether there is another element in the
// current array or object being parsed.
func (dec *Decoder) More() bool {
	_, err := dec.peek()
	return err == nil
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r'
}

func (dec *Decoder) readValue() (int, error) {
	fmt.Println("------- b:readValue --------")
	scanp := dec.scanp

Input:
	for {
		fmt.Println(">> for i, c := range dec.buf[scanp:]")
		for i, c := range dec.buf[scanp:] {
		//fmt.Printf("for %d, %c := range dec.buf[%d]\n", i, c, len(dec.buf[scanp:]))
			switch c {
			case '\n':
				fmt.Println(">> scanp += i")
				scanp += i
				fmt.Printf("%d += %d\n", scanp, i)
				break Input
			}
		}
	}
	fmt.Println(">> return scanp - dec.scanp, nil")
	fmt.Printf("%d, nil \n", scanp - dec.scanp)
	fmt.Println("------- e:readValue --------")
	return scanp - dec.scanp, nil
}

const (
	tokenNewLineValue = iota
	tokenNewLine
)

func (dec *Decoder) tokenPrepareForDecode() error {
	switch dec.tokenState {
	case tokenNewLine:
		dec.scanp++
		dec.tokenState = tokenNewLineValue
	}
	return nil
}
