# CSV Stream


The actual implementation is inspired by the Go JSON decoder and CSV reader, so it can read records from a reader (e.g: file, connection) and is fully compatible with the existing csv parser (passes the same tests).
 
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
