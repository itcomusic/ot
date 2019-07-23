package main

import (
	"context"
	"log"
	"math"

	"github.com/itcomusic/ot"
)

func main() {
	w, attr, err := ot.OpenFile("path")
	if err != nil {
		log.Fatal(err)
	}

	attr.NodeID = math.MaxInt64 // node id
	if err := ot.NewEndpoint("127.0.0.1").
		User("test", "test").
		AddVersionFile(context.Background(), attr, w); err != nil {
		log.Fatal(err)
	}
	log.Println("added new version file")
}
