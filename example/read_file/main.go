package main

import (
	"context"
	"log"
	"math"
	"os"

	"github.com/itcomusic/ot"
)

func main() {
	f, err := os.Create("test-file")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	attr, err := ot.NewEndpoint("127.0.0.1").
		User("test", "test").
		ReadFile(context.Background(), 0, math.MaxInt64, f)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("attributes of the file, %v", attr)
}
