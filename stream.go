package csv

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	
)

type SyntaxError struct {
	msg    string // description of error
	Offset int64  // error occurred after reading Offset bytes
}

func (e *SyntaxError) Error() string { return e.msg }

// A Decoder reads and decodes CSV values from an input stream.
type Decoder struct {
	// FieldsPerRecord is the number of expected fields per record.
	// If FieldsPerRecord is positive, Read requires each record to
	// have the given number of fields. If FieldsPerRecord is 0, Read sets it to
	// the number of fields in the first record, so that future records must
	// have the same field count. If FieldsPerRecord is negative, no check is
	// made and records may have a variable number of fields.
	FieldsPerRecord int
	
	TrailingComma bool // ignored; here for backwards compatibility
	
	// ReuseRecord controls whether calls to Read may return a slice sharing
	// the backing array of the previous call's returned slice for performance.
	// By default, each call to Read returns newly allocated memory owned by the caller.
	ReuseRecord bool
	
	line   int
	column int
	
	r *bufio.Reader
	
	buf   []byte
	d     decodeState
	scanp int // start of unread data in buf
	scan  scanner
	err   error
	
	// lineBuffer holds the unescaped fields read by readField, one after another.
	// The fields can be accessed by using the indexes in fieldIndexes.
	// Example: for the row `a,"b","c""d",e` lineBuffer will contain `abc"de` and
	// fieldIndexes will contain the indexes 0, 1, 2, 5.
	lineBuffer bytes.Buffer
	// Indexes of fields inside lineBuffer
	// The i'th field starts at offset fieldIndexes[i] in lineBuffer.
	fieldIndexes []int
	
	tokenState int
	tokenStack []int
}

// NewDecoder returns a new decoder that reads from r.
//
// The decoder introduces its own buffering and may
// read data from r beyond the CSV values requested.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		scan: scanner{
			Delimiter: ',',
		},
		r: bufio.NewReader(r),
	}
}

// More reports whether there is another element in the
// current array or object being parsed.
func (d *Decoder) More() bool {
	_, err := d.peek()
	return err == nil && d.scan.err == nil
}

func (d *Decoder) Decode() (fields []string, err error) {
	// unexpected error
	if d.err != nil {
		return nil, d.err
	}
	
	// Reset the previous line and truncate the indexes slice
	d.lineBuffer.Reset()
	d.fieldIndexes = d.fieldIndexes[:0]
	
	// Parse the existing buffered data
	n, err := d.readRecord()
	if err != nil {
		if err == io.EOF {
			if len(d.fieldIndexes) == 0 {
				err = io.ErrUnexpectedEOF
			}
		}
		d.err = err
		return nil, err
	}
	
	d.scanp += n
	
	// Creates room for the individual fields
	fieldCount := len(d.fieldIndexes)
	fields = make([]string, fieldCount)
	
	// Break down the fields in the line with the help of
	// the indexes map
	line := d.lineBuffer.String()
	
	for i, idx := range d.fieldIndexes {
		if i == fieldCount-1 {
			fields[i] = line[idx:]
		} else {
			fields[i] = line[idx:d.fieldIndexes[i+1]]
		}
	}
	
	if d.FieldsPerRecord > 0 {
		if len(fields) != d.FieldsPerRecord {
			//r.column = 0 // report at start of record
			d.err = ErrFieldCount
			return fields, d.err
		}
	} else if d.FieldsPerRecord == 0 {
		d.FieldsPerRecord = len(fields)
	}
	
	return fields, nil
}

// returns when a record is present
func (d *Decoder) readRecord() (int, error) {
	d.scan.reset()
	
	scanp := d.scanp
	var err error
	
	d.fieldIndexes = append(d.fieldIndexes, 0)
Input:
	for {
		// Look in the buffer for a new value.
		for i, c := range d.buf[scanp:] {
			d.scan.bytes++
			v := d.scan.step(&d.scan, c)
			
			if v != scanFieldDelimiter && v != scanEndRecord && v != scanSkipSpace && v != scanError {
				d.lineBuffer.WriteByte(c)
			}
			
			if v == scanFieldDelimiter {
				d.fieldIndexes = append(d.fieldIndexes, d.lineBuffer.Len())
			}
			
			if v == scanEnd {
				scanp += i
				break Input
			}
			
			if v == scanEndRecord /*&& d.scan.step(&d.scan, ' ') == scanEnd */ {
				if d.scan.redo {
					d.lineBuffer.Truncate(d.lineBuffer.Len() - 1)
				}
				scanp += i + 1
				break Input
			}
			
			if v == scanError {
				d.err = d.scan.err
				return 0, d.scan.err
			}
			
		}
		scanp = len(d.buf)
		
		if err != nil {
			if err == io.EOF {
				d.scanp = scanp
				break Input
			}
			d.err = err
			return 0, err
		}
		
		n := scanp - d.scanp
		err = d.refill()
		scanp = d.scanp + n
	}
	return scanp - d.scanp, nil
}

func nonSpace(b []byte) bool {
	for _, c := range b {
		if !isSpace(c) {
			return true
		}
	}
	return false
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r'
}

// peek checks if there is any data interesting to read.
func (d *Decoder) peek() (byte, error) {
	var err error
	for {
		// scans the buffer from the actual position (read so far)
		// to the end of the existing buffered data
		for i := d.scanp; i < len(d.buf); i++ {
			c := d.buf[i]
			// keep scanning the buffer until it finds something to parse
			if d.isSpace(c) {
				continue
			}
			
			d.scanp = i
			return c, nil
		}
		
		// buffer has been scanned, now report any error
		if err != nil {
			return 0, err
		}
		
		// there is no more data to scan in the buffer
		// refill will buffer more data if exists
		// the error is kept until next iteration
		err = d.refill()
	}
}

func (d *Decoder) refill() error {
	// Make room to read more into the buffer.
	// First slide down data already consumed.
	if d.scanp > 0 {
		n := copy(d.buf, d.buf[d.scanp:])
		d.buf = d.buf[:n]
		d.scanp = 0
	}
	
	// Grow buffer if not large enough.
	const minRead = 512
	if cap(d.buf)-len(d.buf) < minRead {
		newBuf := make([]byte, len(d.buf), 2*cap(d.buf)+minRead)
		copy(newBuf, d.buf)
		d.buf = newBuf
	}
	
	// Read. Delay error for next iteration (after scan).
	n, err := d.r.Read(d.buf[len(d.buf):cap(d.buf)])
	d.buf = d.buf[0: len(d.buf)+n]
	return err
}

func (d *Decoder) isSpace(c byte) bool {
	if !d.scan.TrimLeadingSpace {
		return c == '\t' || c == '\r' || c == '\n'
	}
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// A ParseError is returned for parsing errors.
// The first line is 1.  The first column is 0.
type ParseError struct {
	Line   int   // Line where the error occurred
	Column int   // Column (rune index) where the error occurred
	Err    error // The actual error
}

// error creates a new ParseError based on err.
func (d *Decoder) error(err error) error {
	return &ParseError{
		Line:   d.line,
		Column: d.column,
		Err:    err,
	}
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Err)
}
