package oscript_test

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/itcomusic/ot/pkg/oscript"
)

func ExampleMarshal() {
	type ColorGroup struct {
		ID     int
		Name   string
		Colors []string
	}
	group := ColorGroup{
		ID:     1,
		Name:   "Reds",
		Colors: []string{"Crimson", "Red", "Ruby", "Maroon"},
	}
	b, err := oscript.Marshal(group)
	if err != nil {
		fmt.Println("error:", err)
	}
	os.Stdout.Write(b)
	// Output:
	// A<1,?,'ID'=1,'Name'='Reds','Colors'={'Crimson','Red','Ruby','Maroon'}>
}

func ExampleUnmarshal() {
	var oscriptBlob = []byte(`{
	A<1,?,'Name'= 'Platypus', 'Order'= 'Monotremata'>,
	A<1,?,'Name'= 'Quoll',    'Order'= 'Dasyuromorphia'>
}`)
	type Animal struct {
		Name  string
		Order string
	}

	var animals []Animal
	err := oscript.Unmarshal(oscriptBlob, &animals)

	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%+v", animals)
	// Output:
	// [{Name:Platypus Order:Monotremata} {Name:Quoll Order:Dasyuromorphia}]
}

// This example uses a Decoder to decode a stream of distinct Oscript values.
func ExampleDecoder() {
	const oscriptStream = `
	A<1,?,'Name'= 'Ed', 'Text'= 'Knock knock.'>
 	A<1,?,'Name'= 'Sam', 'Text'= 'Who\'s there?'>
	A<1,?,'Name'= 'Ed', 'Text'= 'Go fmt.'>
	A<1,?,'Name'= 'Sam', 'Text'= 'Go fmt who?'>
	A<1,?,'Name'= 'Ed', 'Text'= 'Go fmt yourself!'>
`
	type Message struct {
		Name, Text string
	}
	dec := oscript.NewDecoder(strings.NewReader(oscriptStream))

	for {
		var m Message
		if err := dec.Decode(&m); err == io.EOF {
			break

		} else if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: %s\n", m.Name, m.Text)
	}

	// Output:
	// Ed: Knock knock.
	// Sam: Who's there?
	// Ed: Go fmt.
	// Sam: Go fmt who?
	// Ed: Go fmt yourself!
}

// This example uses a Decoder to decode a stream of distinct Oscript values.
func ExampleDecoder_Token() {
	const oscriptStream = `
	A<1,?,'Message'= 'Hello', 'Array'= {1, 2, 3}, 'Undefined'= ?, 'Number'= G1.234>
`
	dec := oscript.NewDecoder(strings.NewReader(oscriptStream))

	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%T: %v", t, t)

		if dec.More() {
			fmt.Printf(" (more)")
		}
		fmt.Printf("\n")
	}
	// Output:
	// oscript.Delim: A (more)
	// string: Message (more)
	// string: Hello (more)
	// string: Array (more)
	// oscript.Delim: { (more)
	// int64: 1 (more)
	// int64: 2 (more)
	// int64: 3
	// oscript.Delim: } (more)
	// string: Undefined (more)
	// <nil>: <nil> (more)
	// string: Number (more)
	// float64: 1.234
	// oscript.Delim: >
}

// This example uses a Decoder to decode a streaming array of Oscript objects.
func ExampleDecoder_Decode_stream() {
	const oscriptStream = `
	{
		A<1,?,'Name'= 'Ed', 'Text'= 'Knock knock.'>,
		A<1,?,'Name'= 'Sam', 'Text'= 'Who\'s there?'>,
		A<1,?,'Name'= 'Ed', 'Text'= 'Go fmt.'>,
		A<1,?,'Name'= 'Sam', 'Text'= 'Go fmt who?'>,
		A<1,?,'Name'= 'Ed', 'Text'= 'Go fmt yourself!'>
	}
`
	type Message struct {
		Name, Text string
	}

	dec := oscript.NewDecoder(strings.NewReader(oscriptStream))
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

	// Output:
	// oscript.Delim: {
	// Ed: Knock knock.
	// Sam: Who's there?
	// Ed: Go fmt.
	// Sam: Go fmt who?
	// Ed: Go fmt yourself!
	// oscript.Delim: }
}
