package main

import (
	"bufio"
	"fmt"
	"os"
	
	csvstream "github.com/calvernaz/csv-stream"
)

func readCsv() {
	file, _ := os.Open("./sample.csv")
	dec := csvstream.NewDecoder(bufio.NewReader(file))
	for dec.More() {
		record, _ := dec.Decode()
		fmt.Println(record)
	}
}


func main() {
	readCsv()
}
