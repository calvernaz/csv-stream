package csv

import (
	"errors"
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

type scanner struct {
	// Delimiter is the field delimiter.
	// It is set to comma (',') by NewReader.
	Delimiter byte
	// If TrimLeadingSpace is true, leading white space in a field is ignored.
	// This is done even if the field delimiter, Delimiter, is white space.
	TrimLeadingSpace bool
	// Comment, if not 0, is the comment character. Lines beginning with the
	// Comment character without preceding whitespace are ignored.
	// With leading whitespace the Comment character becomes part of the
	// field, even if TrimLeadingSpace is true.
	Comment byte
	// If LazyQuotes is true, a quote may appear in an unquoted field and a
	// non-doubled quote may appear in a quoted field.
	LazyQuotes bool
	
	step       func(*scanner, byte) int
	
	// Error that happened, if any.
	err error
	
	// 1-byte redo (see undo method)
	redo      bool
	redoState func(*scanner, byte) int
	
	// total bytes consumed, updated by decoder.Decode
	bytes int64
}

const (
	scanContinue       = iota // uninteresting byte
	scanBeginField
	scanFieldDelimiter  // field delimiter
	scanSkip            // space byte; can skip
	scanEndRecord       // end of record
	scanCarriageReturn
	scanBareQuotes
	
	// Stop
	scanError  // hit an error, scanner.err
)

// reset prepares the scanner for use.
// It must be called before calling s.step.
func (s *scanner) reset() {
	s.step = stateBeginValue
	s.err = nil
	s.redo = false
}

func stateBeginComment(s *scanner, c byte) int {
	if c == '\n' {
		s.step = stateBeginValue
		return scanSkip
	}
	return scanSkip
}

// stateBeginValue is the state at the beginning of the input.
func stateBeginValue(s *scanner, c byte) int {
	if c == ' ' && s.TrimLeadingSpace {
		return scanSkip
	}
	
	if c == s.Comment {
		s.step = stateBeginComment
		return scanSkip
	}
	
	// fields either can be in form of a string or text
	switch c {
	case s.Delimiter:
	case '"':
		s.step = stateInQuotedField
		return scanSkip
	case '\n':
		return scanEndRecord
	default:
		s.step = stateInUnquotedField
		return scanBeginField
	}
	
	if s.err != nil {
		if s.err == io.EOF {
			return scanFieldDelimiter
		}
		return scanSkip
	}
	
	return scanFieldDelimiter
}

func stateCarriageReturn(s *scanner, c byte) int {
	if s.TrimLeadingSpace && c != '\n' && unicode.IsSpace(rune(c)) {
		s.step = stateCarriageReturn
		return scanSkip
	}
	
	if c == '\n' {
		return stateEndValue(s, c)
	}
	
	s.step = s.redoState
	return scanCarriageReturn
}

func stateBareQuote(s *scanner, c byte) int {
	if c == s.Delimiter {
		return stateEndValue(s, c)
	}
	
	if c == '\n' {
		s.step = stateBeginValue
		return stateEndValue(s, c)
	}
	
	if c != '"' {
		if !s.LazyQuotes {
			s.err = ErrQuote
			return scanError
		}
		s.step = stateInQuotedField
		return scanBareQuotes
	}
	
	s.step = stateInQuotedField
	return scanContinue
}

func stateInQuotedField(s *scanner, c byte) int {
	
	if c == '"' {
		s.step = stateBareQuote
		return scanSkip
	}
	return scanContinue
}

func stateInUnquotedField(s *scanner, c byte) int {
	if c == s.Delimiter {
		s.step = stateBeginValue
		return stateBeginValue(s, c)
	}
	
	if c == '\r' {
		s.redoState = stateInUnquotedField
		s.step = stateCarriageReturn
		return scanSkip
	}
	
	if c == '\n' {
		s.step = stateBeginValue
		return scanEndRecord
	}
	
	if !s.LazyQuotes && c == '"' {
		s.err = ErrBareQuote
		return scanError
	}
	
	return scanContinue
}

func stateEndValue(s *scanner, c byte) int {
	if c == s.Delimiter {
		s.step = stateBeginValue
		return scanFieldDelimiter
	}
	return scanEndRecord
}
