package main

import (
	"math/rand"
	"time"

	"git.digineo.de/digineo/zackup/cmd"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	cmd.Execute()
}
