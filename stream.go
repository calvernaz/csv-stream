package csv

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode"
	
)

// These are the errors that can be returned in ParseError.Error
var (
	ErrTrailingComma = errors.New("extra delimiter at end of line") // no longer used
	ErrBareQuote     = errors.New("bare \" in non-quoted-field")
	ErrQuote         = errors.New("extraneous \" in field")
	ErrFieldCount    = errors.New("wrong number of fields in line")
)

type SyntaxError struct {
	msg    string // description of error
	Offset int64  // error occurred after reading Offset bytes
}

func (e *SyntaxError) Error() string { return e.msg }

// A Decoder reads and decodes CSV values from an input stream.
type Decoder struct {
	// Delimiter is the field delimiter.
	// It is set to comma (',') by NewReader.
	Delimiter byte
	// Comment, if not 0, is the comment character. Lines beginning with the
	// Comment character without preceding whitespace are ignored.
	// With leading whitespace the Comment character becomes part of the
	// field, even if TrimLeadingSpace is true.
	Comment byte
	// FieldsPerRecord is the number of expected fields per record.
	// If FieldsPerRecord is positive, Read requires each record to
	// have the given number of fields. If FieldsPerRecord is 0, Read sets it to
	// the number of fields in the first record, so that future records must
	// have the same field count. If FieldsPerRecord is negative, no check is
	// made and records may have a variable number of fields.
	FieldsPerRecord int
	// If LazyQuotes is true, a quote may appear in an unquoted field and a
	// non-doubled quote may appear in a quoted field.
	LazyQuotes    bool
	TrailingComma bool // ignored; here for backwards compatibility
	// If TrimLeadingSpace is true, leading white space in a field is ignored.
	// This is done even if the field delimiter, Delimiter, is white space.
	TrimLeadingSpace bool
	// ReuseRecord controls whether calls to Read may return a slice sharing
	// the backing array of the previous call's returned slice for performance.
	// By default, each call to Read returns newly allocated memory owned by the caller.
	ReuseRecord bool
	
	line   int
	column int
	
	r *bufio.Reader
	
	rr *bufio.Reader
	
	// lineBuffer holds the unescaped fields read by readField, one after another.
	// The fields can be accessed by using the indexes in fieldIndexes.
	// Example: for the row `a,"b","c""d",e` lineBuffer will contain `abc"de` and
	// fieldIndexes will contain the indexes 0, 1, 2, 5.
	lineBuffer bytes.Buffer
	
	// Indexes of fields inside lineBuffer
	// The i'th field starts at offset fieldIndexes[i] in lineBuffer.
	fieldIndexes []int
	
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
	return &Decoder{
		Delimiter: ',',
		r:         bufio.NewReader(r),
	}
}

// More reports whether there is another element in the
// current array or object being parsed.
func (d *Decoder) More() bool {
	_, err := d.peek()
	return err == nil
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
	err = d.decodeBuffer()
	if err != nil {
		if err == io.EOF {
			if len(d.fieldIndexes) != 0 {
				err = io.ErrUnexpectedEOF
			}
		}
		d.err = err
		return nil, err
	}
	
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
	
	return fields, nil
}

const (
	delimiterToken = iota
	newLineToken
	carriageReturnToken
	quotesToken
	fieldToken
	commentToken
)

func (d *Decoder) decodeBuffer() (err error) {
	
	scanp := d.scanp
	
	// at least one index
	d.tokenStack = d.tokenStack[:0]
	d.fieldIndexes = append(d.fieldIndexes, d.lineBuffer.Len())
Input:
	for {
		for _, r := range d.buf[scanp:] {
			switch r {
			case d.Delimiter:
				if len(d.tokenStack) > 0 && d.tokenStack[len(d.tokenStack)-1] == quotesToken {
					if d.tokenStack[len(d.tokenStack)-1] != commentToken {
						d.lineBuffer.WriteByte(r)
					}
				} else {
					if len(d.tokenStack) > 0 {
						if  d.tokenStack[len(d.tokenStack)-1] != commentToken {
							d.fieldIndexes = append(d.fieldIndexes, d.lineBuffer.Len())
							d.tokenState = delimiterToken
						}
					} else {
						d.fieldIndexes = append(d.fieldIndexes, d.lineBuffer.Len())
						d.tokenState = delimiterToken
					}
				}
			case d.Comment:
				d.tokenState = commentToken
				d.tokenStack = append(d.tokenStack, d.tokenState)
			case '"':
				// end of double-quote field
				if len(d.tokenStack) > 0 && d.tokenStack[len(d.tokenStack)-1] == quotesToken {
					d.tokenState = quotesToken
					d.tokenStack = d.tokenStack[:len(d.tokenStack)-1]
				} else { // first double-quote field
					if d.tokenState == quotesToken {
						d.lineBuffer.WriteRune('"')
						d.tokenStack = append(d.tokenStack, d.tokenState)
					} else {
						if d.LazyQuotes && d.tokenState == fieldToken {
							d.lineBuffer.WriteRune('"')
						} else {
							d.tokenState = quotesToken
							d.tokenStack = append(d.tokenStack, d.tokenState)
						}
					}
				}
			case '\n':
				d.tokenState = newLineToken
				if len(d.tokenStack) > 0 && d.tokenStack[len(d.tokenStack)-1] == carriageReturnToken {
					scanp++
					d.lineBuffer.Truncate(d.lineBuffer.Len()-1)
					break Input
				} else if len(d.tokenStack) > 0 && d.tokenStack[len(d.tokenStack)-1] == quotesToken {
					if d.tokenStack[len(d.tokenStack)-1] != commentToken {
						d.lineBuffer.WriteByte(r)
					}
				} else {
					scanp++
					if len(d.tokenStack) > 0 && d.tokenStack[len(d.tokenStack)-1] == commentToken {
						d.tokenStack = d.tokenStack[:0]
						continue
					}
					break Input
				}
			case '\r':
				if len(d.tokenStack) > 0 && d.tokenStack[len(d.tokenStack)-1] != carriageReturnToken {
					d.tokenState = carriageReturnToken
					d.tokenStack = append(d.tokenStack, d.tokenState)
				} else {
					d.tokenState = carriageReturnToken
					d.tokenStack = append(d.tokenStack, d.tokenState)
				}
				fallthrough
			default:
				d.tokenState = fieldToken
				if d.TrimLeadingSpace {
					if !unicode.IsSpace(rune(r)) {
						if len(d.tokenStack) > 0 {
							if d.tokenStack[len(d.tokenStack)-1] != commentToken {
								d.lineBuffer.WriteByte(r)
							}
						} else {
							d.lineBuffer.WriteByte(r)
						}
					}
				} else {
					if len(d.tokenStack) > 0 {
						if d.tokenStack[len(d.tokenStack)-1] != commentToken {
							d.lineBuffer.WriteByte(r)
						}
					} else {
						d.lineBuffer.WriteByte(r)
					}
				}
				
			}
			scanp++
		}
		
		//scanp = len(d.buf)
		
		// Did the last read have an error?
		// Delayed until now to allow buffer scan.
		if err != nil {
			if err == io.EOF {
				break Input
			}
			d.err = err
			return err
		}
		
		n := scanp - d.scanp
		err = d.refill()
		scanp = d.scanp + n
	}
	
	d.scanp += scanp - d.scanp
	
	return nil
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
			if c == d.Comment {
				d.tokenState = commentToken
				d.tokenStack = append(d.tokenStack, d.tokenState)
				continue
			}
			if c != '\n' {
				if len(d.tokenStack) > 0 && d.tokenStack[len(d.tokenStack)-1] == commentToken {
					continue
				}
			} else {
				if d.isSpace(c) {
					continue
				}
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
	if !d.TrimLeadingSpace {
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
