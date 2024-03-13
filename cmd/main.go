package main

import (
	"log"

	"github.com/eczy/ghforeach/internal/ghforeach"
)

func main() {
	if err := ghforeach.Run(); err != nil {
		log.Fatal(err)
	}
}
