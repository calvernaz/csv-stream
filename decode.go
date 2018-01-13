package csv

// decodeState represents the state while decoding a CSV record.
type decodeState struct {
	data []byte
	off int
	scan scanner
	nextscan scanner
	errorContext struct {
		Struct string
		Field string
	}
	
	savedError error
	useNumber bool
}
