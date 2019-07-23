package oscript_test

import (
	"fmt"
	"log"
	"strings"

	"github.com/itcomusic/ot/pkg/oscript"
)

type Animal int

const (
	Unknown Animal = iota
	Gopher
	Zebra
)

func (a *Animal) UnmarshalOscript(b []byte) error {
	var s string

	if err := oscript.Unmarshal(b, &s); err != nil {
		return err
	}

	switch strings.ToLower(s) {
	default:
		*a = Unknown

	case "gopher":
		*a = Gopher

	case "zebra":
		*a = Zebra
	}
	return nil
}

func (a Animal) MarshalOscript() ([]byte, error) {
	var s string
	switch a {
	default:
		s = "unknown"

	case Gopher:
		s = "gopher"

	case Zebra:
		s = "zebra"
	}
	return oscript.Marshal(s)
}

func Example_customMarshalOscript() {
	blob := `{'gopher','armadillo','zebra','unknown','gopher','bee','gopher','zebra'}`
	var zoo []Animal

	if err := oscript.Unmarshal([]byte(blob), &zoo); err != nil {
		log.Fatal(err)
	}

	census := make(map[Animal]int)
	for _, animal := range zoo {
		census[animal]++
	}

	fmt.Printf("Zoo Census:\n* Gophers: %d\n* Zebras:  %d\n* Unknown: %d\n",
		census[Gopher], census[Zebra], census[Unknown])
	// Output:
	// Zoo Census:
	// * Gophers: 3
	// * Zebras:  2
	// * Unknown: 3
}
