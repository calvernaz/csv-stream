[![Build Status](https://travis-ci.org/calvernaz/csv-stream.svg?branch=master)](https://travis-ci.org/calvernaz/csv-stream)
[![Coverage Status](https://coveralls.io/repos/github/calvernaz/csv-stream/badge.svg?branch=master)](https://coveralls.io/github/calvernaz/csv-stream?branch=master)

# csv-stream

`csv-stream` uses a lookahead parser inspired by the Go JSON decoder and CSV reader, so it can read records from a reader
(e.g: file, connection) and is fully compatible with the existing csv parser (passes the same tests).

Ideal for streaming csv records across the network without carrying about how is the data structured.

# Usage

### Reading records from a file

```go
file, _ := os.Open("./sample.csv")
dec := csvstream.NewDecoder(bufio.NewReader(file))

for dec.More() {
	record, _ := dec.Decode()

	fmt.Println(record)
}
````

### Reading records from a connection

```go
func decode(c *websocket.Conn) {
	dec = csvstream.NewDecoder(c)

	for dec.More() {
		record, _ := dec.Decode()
		fmt.Println(record)
	}
}
```

# Benchmarks

The benchmark tests are the same as the ones in the standard library,

### csv-stream

```go
BenchmarkRead-4                         	 1000000	      2061 ns/op	     664 B/op	      18 allocs/op
BenchmarkReadWithFieldsPerRecord-4      	 1000000	      2116 ns/op	     664 B/op	      18 allocs/op
BenchmarkReadWithoutFieldsPerRecord-4   	 1000000	      2137 ns/op	     664 B/op	      18 allocs/op
BenchmarkReadLargeFields-4              	   50000	     37344 ns/op	    3936 B/op	      24 allocs/op
```

### standard library csv

```go
BenchmarkRead-4                                    	  500000	      3140 ns/op	     664 B/op	      18 allocs/op
BenchmarkReadWithFieldsPerRecord-4                 	  500000	      3131 ns/op	     664 B/op	      18 allocs/op
BenchmarkReadWithoutFieldsPerRecord-4              	  500000	      3117 ns/op	     664 B/op	      18 allocs/op
BenchmarkReadLargeFields-4                         	   20000	     67522 ns/op	    3936 B/op	      24 allocs/op
```

The tests show a speedup of approximately 30%.   
