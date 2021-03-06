package csv

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

var readTests = []struct {
	Name               string
	Input              string
	Output             [][]string
	UseFieldsPerRecord bool // false (default) means FieldsPerRecord is -1
	
	// These fields are copied into the Reader
	Delimiter        rune
	Comment          rune
	FieldsPerRecord  int
	LazyQuotes       bool
	TrailingComma    bool
	TrimLeadingSpace bool
	ReuseRecord      bool
	
	Error  string
	Line   int // Expected error line if != 0
	Column int // Expected error column if line != 0
}{
	{
		Name:   "Simple",
		Input:  "a,b,c\n",
		Output: [][]string{{"a", "b", "c"}},
	},
	{
		Name:   "CRLF",
		Input:  "a,b\r\nc,d\r\n",
		Output: [][]string{{"a", "b"}, {"c", "d"}},
	},
	{
		Name:   "BareCR",
		Input:  "a,b\rc,d\r\n",
		Output: [][]string{{"a", "b\rc", "d"}},
	},
	{
		Name:               "RFC4180test",
		UseFieldsPerRecord: true,
		Input: `#field1,field2,field3
"aaa","bb
b","ccc"
"a,a","b""bb","ccc"
zzz,yyy,xxx
`,
		Output: [][]string{
			{"#field1", "field2", "field3"},
			{"aaa", "bb\nb", "ccc"},
			{"a,a", `b"bb`, "ccc"},
			{"zzz", "yyy", "xxx"},
		},
	},
	{
		Name:   "NoEOLTest",
		Input:  "a,b,c",
		Output: [][]string{{"a", "b", "c"}},
	},
	{
		Name:      "Semicolon",
		Delimiter: ';',
		Input:     "a;b;c\n",
		Output:    [][]string{{"a", "b", "c"}},
	},
	{
		Name: "MultiLine",
		Input: `"two
line","one line","three
line
field"`,
		Output: [][]string{{"two\nline", "one line", "three\nline\nfield"}},
	},
	{
		Name:  "BlankLine",
		Input: "a,b,c\n\nd,e,f\n\n",
		Output: [][]string{
			{"a", "b", "c"},
			{"d", "e", "f"},
		},
	},
	{
		Name:               "BlankLineFieldCount",
		Input:              "a,b,c\n\nd,e,f\n\n",
		UseFieldsPerRecord: true,
		Output: [][]string{
			{"a", "b", "c"},
			{"d", "e", "f"},
		},
	},
	{
		Name:             "TrimSpace",
		Input:            " a,  b,   c\n",
		TrimLeadingSpace: true,
		Output:           [][]string{{"a", "b", "c"}},
	},
	{
		Name:   "LeadingSpace",
		Input:  " a,  b,   c\n",
		Output: [][]string{{" a", "  b", "   c"}},
	},
//	{
//		Name:    "Comment",
//		Comment: '#',
//		Input:   "#1,2,3\na,b,c\n#comment",
//		Output:  [][]string{{"a", "b", "c"}},
//	},
	{
		Name:   "NoComment",
		Input:  "#1,2,3\na,b,c",
		Output: [][]string{{"#1", "2", "3"}, {"a", "b", "c"}},
	},
	{
		Name:       "LazyQuotes",
		LazyQuotes: true,
		Input:      `a "word","1"2",a","b`,
		Output:     [][]string{{`a "word"`, `1"2`, `a"`, `b`}},
	},
	{
		Name:       "BareQuotes",
		LazyQuotes: true,
		Input:      `a "word","1"2",a"`,
		Output:     [][]string{{`a "word"`, `1"2`, `a"`}},
	},
	{
		Name:       "BareDoubleQuotes",
		LazyQuotes: true,
		Input:      `a""b,c`,
		Output:     [][]string{{`a""b`, `c`}},
	},
	{
		Name:  "BadDoubleQuotes",
		Input: `a""b,c`,
		Error: `bare " in non-quoted-field`, Line: 1, Column: 1,
	},
	{
		Name:             "TrimQuote",
		Input:            ` "a"," b",c`,
		TrimLeadingSpace: true,
		Output:           [][]string{{"a", " b", "c"}},
	},
	{
		Name:  "BadBareQuote",
		Input: `a "word","b"`,
		Error: `bare " in non-quoted-field`, Line: 1, Column: 2,
	},
	{
		Name:  "BadTrailingQuote",
		Input: `"a word",b"`,
		Error: `bare " in non-quoted-field`, Line: 1, Column: 10,
	},
	{
		Name:  "ExtraneousQuote",
		Input: `"a "word","b"`,
		Error: `extraneous " in field`, Line: 1, Column: 3,
	},
	{
		Name:               "BadFieldCount1",
		UseFieldsPerRecord: true,
		FieldsPerRecord:    2,
		Input:              `a,b,c`,
		Error:              "wrong number of fields", Line: 1,
	},
	{
		Name:   "FieldCount",
		Input:  "a,b,c\nd,e",
		Output: [][]string{{"a", "b", "c"}, {"d", "e"}},
	},
	{
		Name:   "TrailingCommaEOF",
		Input:  "a,b,c,",
		Output: [][]string{{"a", "b", "c", ""}},
	},
	{
		Name:   "TrailingCommaEOL",
		Input:  "a,b,c,\n",
		Output: [][]string{{"a", "b", "c", ""}},
	},
	{
		Name:             "TrailingCommaSpaceEOF",
		TrimLeadingSpace: true,
		Input:            "a,b,c, ",
		Output:           [][]string{{"a", "b", "c", ""}},
	},
	{
		Name:             "TrailingCommaSpaceEOL",
		TrimLeadingSpace: true,
		Input:            "a,b,c, \n",
		Output:           [][]string{{"a", "b", "c", ""}},
	},
	{
		Name:             "TrailingCommaLine3",
		TrimLeadingSpace: true,
		Input:            "a,b,c\nd,e,f\ng,hi,",
		Output:           [][]string{{"a", "b", "c"}, {"d", "e", "f"}, {"g", "hi", ""}},
	},
	{
		Name:   "NotTrailingComma3",
		Input:  "a,b,c, \n",
		Output: [][]string{{"a", "b", "c", " "}},
	},
	{
		Name:          "CommaFieldTest",
		TrailingComma: true,
		Input: `x,y,z,w
x,y,z,
x,y,,
x,,,
,,,
"x","y","z","w"
"x","y","z",""
"x","y","",""
"x","","",""
"","","",""
`,
		Output: [][]string{
			{"x", "y", "z", "w"},
			{"x", "y", "z", ""},
			{"x", "y", "", ""},
			{"x", "", "", ""},
			{"", "", "", ""},
			{"x", "y", "z", "w"},
			{"x", "y", "z", ""},
			{"x", "y", "", ""},
			{"x", "", "", ""},
			{"", "", "", ""},
		},
	},
	{
		Name:             "TrailingCommaIneffective1",
		TrailingComma:    true,
		TrimLeadingSpace: true,
		Input:            "a,b,\nc,d,e",
		Output: [][]string{
			{"a", "b", ""},
			{"c", "d", "e"},
		},
	},
	{
		Name:             "TrailingCommaIneffective2",
		TrailingComma:    false,
		TrimLeadingSpace: true,
		Input:            "a,b,\nc,d,e",
		Output: [][]string{
			{"a", "b", ""},
			{"c", "d", "e"},
		},
	},
	{
		Name:        "ReadAllReuseRecord",
		ReuseRecord: true,
		Input:       "a,b\nc,d",
		Output: [][]string{
			{"a", "b"},
			{"c", "d"},
		},
	},
}

func TestRead(t *testing.T) {
	for _, tt := range readTests {
		r := NewDecoder(strings.NewReader(tt.Input))
		r.scan.Comment = byte(tt.Comment)
		if tt.UseFieldsPerRecord {
			r.FieldsPerRecord = tt.FieldsPerRecord
		} else {
			r.FieldsPerRecord = -1
		}
		r.scan.LazyQuotes = tt.LazyQuotes
		r.scan.TrimLeadingSpace = tt.TrimLeadingSpace
		
		if tt.Delimiter != 0 {
			r.scan.Delimiter = byte(tt.Delimiter)
		}
		
		
		i := 0
		for r.More() {
 			out, err := r.Decode()

			if tt.Error != "" {
				perr, _ := err.(*ParseError)
				if err == nil || !strings.Contains(err.Error(), tt.Error) {
					t.Errorf("%s: error %v, want error %q", tt.Name, err, tt.Error)
				} else if tt.Line != 0 {
					if tt.Line != perr.Line || tt.Column != perr.Column {
						t.Errorf("%s: error at %d:%d expected %d:%d", tt.Name, perr.Line, perr.Column, tt.Line, tt.Column)
					}
				}
			} else if err != nil {
				t.Errorf("%s: unexpected error %v", tt.Name, err)
			} else if !reflect.DeepEqual(out, tt.Output[i]) {
				t.Errorf("%s: out=%q want %q", tt.Name, out, tt.Output[i])
				return
			}
			i++
		}
	}
}

func TestBadFieldCount(t *testing.T) {
	in  := "a,b,c\nd,e"
	dec := NewDecoder(strings.NewReader(in))

	var err error
	for dec.More() {
		_, err = dec.Decode()
		if err != nil {
			break
		}
	}

	perr, _ := err.(*ParseError)
	if perr == nil {
		t.Errorf("error <nil>, want error %q", perr.Error())
	}
}

// nTimes is an io.Reader which yields the string s n times.
type nTimes struct {
	s   string
	n   int
	off int
}

func (r *nTimes) Read(p []byte) (n int, err error) {
	for {
		if r.n <= 0 || r.s == "" {
			return n, io.EOF
		}
		n0 := copy(p, r.s[r.off:])
		p = p[n0:]
		n += n0
		r.off += n0
		if r.off == len(r.s) {
			r.off = 0
			r.n--
		}
		if len(p) == 0 {
			return
		}
	}
}
// benchmarkRead measures reading the provided CSV rows data.
// initReader, if non-nil, modifies the Reader before it's used.
func benchmarkRead(b *testing.B, initReader func(decoder *Decoder), rows string) {
	b.ReportAllocs()
	
	
	d := NewDecoder(&nTimes{ s: rows, n: b.N })
	//d := &Decoder{ r: &nTimes{ s: rows, n: b.N }}
	//r := NewReader(&nTimes{s: rows, n: b.N})
	if initReader != nil {
		initReader(d)
	}
	for d.More() {
		_, err := d.Decode()
		if err == io.EOF {
			break
		}
		if err != nil {
			b.Fatal(err)
		}
	}
}

const benchmarkCSVData = `x,y,z,w
x,y,z,
x,y,,
x,,,
,,,
"x","y","z","w"
"x","y","z",""
"x","y","",""
"x","","",""
"","","",""
`

func BenchmarkRead(b *testing.B) {
	benchmarkRead(b, nil, benchmarkCSVData)
}

func BenchmarkReadWithFieldsPerRecord(b *testing.B) {
	benchmarkRead(b, func(r *Decoder) { r.FieldsPerRecord = 4 }, benchmarkCSVData)
}

func BenchmarkReadWithoutFieldsPerRecord(b *testing.B) {
	benchmarkRead(b, func(r *Decoder) { r.FieldsPerRecord = -1 }, benchmarkCSVData)
}

func BenchmarkReadLargeFields(b *testing.B) {
	benchmarkRead(b, nil, strings.Repeat(`xxxxxxxxxxxxxxxx,yyyyyyyyyyyyyyyy,zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz,wwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwww,vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv
xxxxxxxxxxxxxxxxxxxxxxxx,yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy,zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz,wwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwww,vvvv
,,zzzz,wwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwww,vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx,yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy,zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz,wwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwww,vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv
`, 3))
}
