package csv

import (
	"errors"
	"io"
	"strconv"
	"unicode"
)

// These are the errors that can be returned in ParseError.Error
var (
	ErrTrailingComma = errors.New("extra delimiter at end of line") // no longer used
	ErrBareQuote     = errors.New("bare \" in non-quoted-field")
	ErrQuote         = errors.New("extraneous \" in field")
	ErrFieldCount    = errors.New("wrong number of fields in line")
)

func Valid(data []byte) bool {
	return checkValid(data, &scanner{}) == nil
}

func checkValid(data []byte, scan *scanner) error {
	scan.reset()
	for _, c := range data {
		scan.bytes++
		if scan.step(scan, c) == scanError {
			return scan.err
		}
	}
	if scan.eof() == scanError {
		return scan.err
	}
	return nil
}

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
	
	// Reached end of top-level value
	endTop bool
	
	// stack
	parseState []int
	
	// Error that happened, if any.
	err error
	
	// 1-byte redo (see undo method)
	redo      bool
	redoCode  int
	redoState func(*scanner, byte) int
	
	// total bytes consumed, updated by decoder.Decode
	bytes int64
}

const (
	scanContinue       = iota // uninteresting byte
	scanBeginField
	scanFieldDelimiter  // field delimiter
	scanSkipSpace       // space byte; can skip
	scanEndRecord       // end of record
	scanCarriageReturn
	scanBareQuotes
	// Stop
	scanEnd    // top-level value ended *before* this byte;
	scanError  // hit an error, scanner.err

)

// These values are stored in the parseState stack.
// They give the current state of a composite value
// being scanned. If the parser is inside a nested value
// the parseState describes the nested state, outermost at entry 0.
const (
	parseFieldValue = iota // parsing field value
)

// reset prepares the scanner for use.
// It must be called before calling s.step.
func (s *scanner) reset() {
	s.step = stateBeginValue
	s.parseState = s.parseState[0:0]
	s.err = nil
	s.redo = false
	s.endTop = false
}

// eof tells the scanner that the end of input has been reached.
// It returns a scan status just as s.step does.
func (s *scanner) eof() int {
	if s.err != nil {
		return scanError
	}
	if s.endTop {
		return scanEnd
	}
	s.step(s, ' ')
	if s.endTop {
		return scanEnd
	}
	if s.err == nil {
		s.err = &SyntaxError{"unexpected end of CSV input", s.bytes}
	}
	return scanError
}

// pushParseState pushes a new parse state p onto the parse stack.
func (s *scanner) pushParseState(p int) {
	s.parseState = append(s.parseState, p)
}

// popParseState pops a parse state (already obtained) off the stack
// and updates s.step accordingly.
func (s *scanner) popParseState() {
	n := len(s.parseState) - 1
	s.parseState = s.parseState[0:n]
	if n == 0 {
		s.step = stateEndTop
		s.endTop = true
	} else {
		s.step = stateEndValue
	}
}

// stateBeginTextOrEmpty is the state after reading a field without double-quotes.
func stateBeginTextOrEmpty(s *scanner, c byte) int {
	if c == s.Delimiter {
		return stateEndValue(s, c)
	}
	
	if c == '\n' {
		return stateEndValue(s, c)
	}
	
	if !s.LazyQuotes && c == '"' {
		s.err = ErrBareQuote
		return scanError
	}
	//return stateBeginTextOrEmpty(s, c)
	return scanContinue
}

func stateBeginComment(s *scanner, c byte) int {
	if c == '\n' {
		s.step = stateBeginValue
		return scanSkipSpace
	}
	return scanSkipSpace
}

// stateBeginValue is the state at the beginning of the input.
func stateBeginValue(s *scanner, c byte) int {
	if c == ' ' && s.TrimLeadingSpace {
		return scanSkipSpace
	}
	
	if c == s.Comment {
		s.step = stateBeginComment
		return scanSkipSpace
	}
	
	// fields either can be in form of a string or text
	switch c {
	case s.Delimiter:
	case '"':
		s.step = stateInQuotedField
		s.pushParseState(parseFieldValue)
		return scanSkipSpace
	case '\r':
		s.step = stateCarriageReturn
		return scanSkipSpace
	case '\n':
		return scanEndRecord
	default:
		s.step = stateInUnquotedField
		s.pushParseState(parseFieldValue)
		return scanBeginField
	}
	
	if s.err != nil {
		if s.err == io.EOF {
			return scanFieldDelimiter
		}
		return scanSkipSpace
	}
	
	return scanFieldDelimiter
	//return s.error(c, "looking for beginning of value")
}

func stateCarriageReturn(s *scanner, c byte) int {
	if s.TrimLeadingSpace && c != '\n' && unicode.IsSpace(rune(c)) {
		s.step = stateCarriageReturn
		return scanSkipSpace
	}
	
	if c == '\n' {
		return stateEndValue(s, c)
	}
	
	s.step = s.redoState
	return scanCarriageReturn
}

// stateInQuotes is the state after reading `"`.
func stateInString(s *scanner, c byte) int {
	if c == '"' {
		s.step = stateInQuotedField
		return scanSkipSpace
	}
	if c == '\\' {
		s.step = stateInStringEsc
		return scanContinue
	}
	
	return scanContinue
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
		return scanSkipSpace
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
		return scanSkipSpace
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

// stateInStringEsc is the state after reading `"\` during a quoted string.
func stateInStringEsc(s *scanner, c byte) int {
	switch c {
	case '"':
		s.step = stateInString
		return scanContinue
	}
	return s.error(c, "in string escape code")
}

func stateEndValue(s *scanner, c byte) int {
	n := len(s.parseState)
	if n == 0 {
		s.step = stateEndTop
		s.endTop = true
		return stateEndTop(s, c)
	}
	
	if c <= ' ' && isSpace(c) {
		s.step = stateEndValue
		return scanSkipSpace
	}
	
	ps := s.parseState[n-1]
	switch ps {
	case parseFieldValue:
		if c == s.Delimiter {
			s.step = stateBeginValue
			s.parseState[n-1] = parseFieldValue
			return scanFieldDelimiter
		}
		if c == '\n' {
			s.popParseState()
			return scanEndRecord
		}
		return s.error(c, "after array element")
	}
	return s.error(c, "")
}

// stateEndTop is the state after finishing the top-level value,
// such as after reading `{}` or `[1,2,3]`.
// Only space characters should be seen now.
func stateEndTop(s *scanner, c byte) int {
	if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
		// Complain about non-space byte on next call.
		s.error(c, "after top-level value")
	}
	return scanEnd
}

// error records an error and switches to the error state.
func (s *scanner) error(c byte, context string) int {
	s.step = stateError
	s.err = &SyntaxError{"invalid character " + quoteChar(c) + " " + context, s.bytes}
	return scanError
}

func stateError(s *scanner, c byte) int {
	return scanError
}

// quoteChar formats c as a quoted character literal
func quoteChar(c byte) string {
	// special cases - different from quoted strings
	if c == '\'' {
		return `'\''`
	}
	if c == '"' {
		return `'"'`
	}
	
	// use quoted string with different quotation marks
	s := strconv.Quote(string(c))
	return "'" + s[1:len(s)-1] + "'"
}
