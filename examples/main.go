package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	
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


func readCsvNonStream() {
	in := `a""b,c`
	r := csv.NewReader(strings.NewReader(in))
	
	r.Read()
}
func main() {
	//readCsv()
	readCsvNonStream()
}
