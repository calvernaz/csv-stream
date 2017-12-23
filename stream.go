package csv

import (
	"io"
	"bufio"
	"bytes"
	"unicode"
)

type SyntaxError struct {
	msg    string // description of error
	Offset int64  // error occurred after reading Offset bytes
}

func (e *SyntaxError) Error() string { return e.msg }

// A Decoder reads and decodes CSV values from an input stream.
type Decoder struct {
	// Delimiter is the field delimiter.
	// It is set to comma (',') by NewDecoder.
	Delimiter rune
	
	r         io.Reader
	buf       []byte
	scanp     int // start of unread data in buf
	err       error
	
	tokenState int
	tokenStack []int
}

// NewDecoder returns a new decoder that reads from r.
//
// The decoder introduces its own buffering and may
// read data from r beyond the CSV values requested.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		Delimiter: ',',
		r:         bufio.NewReader(r),
	}
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
	
	line := bytes.Runes(dec.buf[dec.scanp: dec.scanp+n])
	dec.scanp += n
	
	var fields []string
	p := 0
	for i, c := range line {
		switch {
		case c == dec.Delimiter: // found a separator
			if dec.tokenStack[len(dec.tokenStack)-1] == tokenBeginQuotes {
				continue
			}
			
			if dec.tokenStack[len(dec.tokenStack)-1] == tokenSeparator {
				if len(line[p:i]) == 0 {
					fields = append(fields, "")
					continue
				}
			}
			
			fields = append(fields, string(line[p:i]))
			p = i + 1
			
			dec.tokenState = tokenSeparator
		case c == '"':
			if dec.tokenStack[len(dec.tokenStack)-1] == tokenBeginQuotes {
				dec.tokenStack = dec.tokenStack[:len(dec.tokenStack)-1]
				continue
			}
			dec.tokenState = tokenBeginQuotes
			dec.tokenStack = append(dec.tokenStack, dec.tokenState)
		}
	}
	
	// last field
	fields = append(fields, string(line[p:]))
	// reset
	dec.tokenStack = dec.tokenStack[:0]
	
	return fields, nil
}

func (dec *Decoder) peek() (byte, error) {
	var err error
	
	for {
		for i := dec.scanp; i < len(dec.buf); i++ {
			c := dec.buf[i]
			if unicode.IsSpace(rune(c)) {
				continue
			}
			dec.tokenState = tokenNewLineValue
			dec.scanp = i
			return c, nil
		}
		// buffer has been scanned, now report any error
		if err != nil {
			return 0, err
		}
		err = dec.refill()
	}
}

func (dec *Decoder) refill() error {
	// Make room to read more into the buffer.
	// First slide down data already consumed.
	if dec.scanp > 0 {
		n := copy(dec.buf, dec.buf[dec.scanp:])
		dec.buf = dec.buf[:n]
		dec.scanp = 0
	}
	
	// Grow buffer if not large enough.
	const minRead = 512
	if cap(dec.buf)-len(dec.buf) < minRead {
		newBuf := make([]byte, len(dec.buf), 2*cap(dec.buf)+minRead)
		copy(newBuf, dec.buf)
		dec.buf = newBuf
	}
	
	// Read. Delay error for next iteration (after scan).
	n, err := dec.r.Read(dec.buf[len(dec.buf):cap(dec.buf)])
	dec.buf = dec.buf[0: len(dec.buf)+n]
	return err
}

// More reports whether there is another element in the
// current array or object being parsed.
func (dec *Decoder) More() bool {
	_, err := dec.peek()
	return err == nil
}

func isNewLine(c rune) bool {
	return c == '\n' || c == '\r'
}

func (dec *Decoder) readValue() (int, error) {
	scanp := dec.scanp
	var err error
Input:
	for {
		for i, c := range bytes.Runes(dec.buf[scanp:]) {
			switch {
			case isNewLine(c):
				scanp += i
				dec.tokenState = tokenNewLine
				break Input
			}
		}
		
		scanp = len(dec.buf)
		
		if err != nil {
			dec.err = err
			return 0, err
		}
		
		n := scanp - dec.scanp
		err = dec.refill()
		scanp = dec.scanp + n
	}
	return scanp - dec.scanp, nil
}

// TODO: get better names and cleanup non required values
const (
	tokenNewLineValue = iota
	tokenBeginFields
	tokenNewLine
	tokenSeparator
	tokenBeginQuotes
)

func (dec *Decoder) tokenPrepareForDecode() error {
	switch dec.tokenState {
	case tokenNewLine:
		dec.scanp++
		dec.tokenState = tokenBeginFields
		dec.tokenStack = append(dec.tokenStack, dec.tokenState)
	default: // TODO: cleanup this
		dec.tokenState = tokenBeginFields
		dec.tokenStack = append(dec.tokenStack, dec.tokenState)
	}
	return nil
}
