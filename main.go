package main

import (
	"math/rand"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"

	"git.digineo.de/digineo/zackup/cmd"
)

func main() {
	go handleSIGUSRx()

	rand.Seed(time.Now().UnixNano())
	cmd.Execute()
}

func handleSIGUSRx() {
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGUSR1)

	for range sig {
		pprof.Lookup("goroutine").WriteTo(os.Stderr, 2)
	}
}
