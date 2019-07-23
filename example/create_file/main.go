package main

import (
	"context"
	"log"
	"math"

	"github.com/itcomusic/ot"
)

func main() {
	f, attr, err := ot.OpenFile("path")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// dont forget set parent id
	if err := ot.NewEndpoint("127.0.0.1").
		User("test", "test").
		CreateFile(context.Background(), math.MaxInt64, "name", attr, f); err != nil {
		log.Fatal(err)
	}
	log.Printf("created new file %d", attr.NodeID)
}
