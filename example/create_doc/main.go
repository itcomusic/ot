package main

import (
	"context"
	"log"
	"math"

	"github.com/itcomusic/ot"
)

func main() {
	r, attr, err := ot.OpenFile("path")
	if err != nil {
		log.Fatal(err)
	}

	if err := ot.NewEndpoint("127.0.0.1").
		User("test", "test").
		CreateDocument(context.Background(), ot.Document{
			Parent:         math.MaxInt64,
			Name:           "name",
			Metadata:       ot.Metadata{}, // category
			File:           attr,
			VersionControl: false,
			Reader:         r,
		}); err != nil {
		log.Fatal(err)
	}
	log.Printf("created new document %d", attr.NodeID)
}
