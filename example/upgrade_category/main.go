package main

import (
	"context"
	"log"
	"math"

	"github.com/itcomusic/ot"
)

func main() {
	ss := ot.NewEndpoint("127.0.0.1").User("test", "test")

	cat, err := ss.GetCategory(context.Background(), math.MaxInt64)
	if err != nil {
		log.Fatal(err)
	}

	node, err := ss.GetNode(context.Background(), math.MaxInt64)
	if err != nil {
		log.Fatal(err)
	}

	if err := node.Metadata.Find("category display name").Upgrade(*cat); err != nil {
		log.Fatal(err)
	}
	log.Println("category was upgraded")
}
