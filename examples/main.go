package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	
	csvstream "github.com/calvernaz/csv-stream"
	"os"
	"bufio"
)

func readStreamCsv() {
	in := `first_name,last_name,username
"Rob","Pike",rob
Ken,Thompson,ken
"Robert","Griesemer","gri"
`
	dec := csvstream.NewDecoder(strings.NewReader(in))
	
	// read open bracket
	// while the array contains values
	for dec.More() {
		fmt.Println(dec.Decode())
	}
	
	fmt.Println("no more data")
	
}

func readCsv() {
	in := `first_name,last_name,username
"Rob","Pike",rob
Ken,Thompson,ken
"Robert","Griesemer","gri"
`
	reader := csv.NewReader(strings.NewReader(in))
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("line %v\n", line)
	}
	
}

func jsonStream() {
	const jsonStream = `
	[
		{"Name": "Ed", "Text": "Knock knock."},
		{"Name": "Sam", "Text": "Who's there?"},
		{"Name": "Ed", "Text": "Go fmt."},
		{"Name": "Sam", "Text": "Go fmt who?"},
		{"Name": "Ed", "Text": "Go fmt yourself!"}
	]
`
	type Message struct {
		Name, Text string
	}
	
	dec := json.NewDecoder(strings.NewReader(jsonStream))
	
	// read open bracket
	t, err := dec.Token()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%T: %v\n", t, t)
	
	// while the array contains values
	for dec.More() {
		var m Message
		// decode an array value (Message)
		err := dec.Decode(&m)
		if err != nil {
			log.Fatal(err)
		}
		
		fmt.Printf("%v: %v\n", m.Name, m.Text)
	}
	
	// read closing bracket
	t, err = dec.Token()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%T: %v\n", t, t)
}

func readCsvFileStream() {
	file, err := os.Open("./titanic_data.csv")
	if err != nil {
		panic(err)
	}
	dec := csvstream.NewDecoder(bufio.NewReader(file))
	
	// read open bracket
	// while the array contains values
	for dec.More() {
		fmt.Println(dec.Decode())
		fmt.Println("more data comming")
	}
	
	fmt.Println("no more data")
}


func main() {
	//jsonStream()
	//readCsv()
	//readStreamCsv()
	readCsvFileStream()
}
