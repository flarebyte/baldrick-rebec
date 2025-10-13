package main

import (
	"log"

	"github.com/flarebyte/baldrick-rebec/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
