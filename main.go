package main

import (
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

	"github.com/digineo/zackup/cmd"
)

func main() {
	go handleSIGUSRx()

	cmd.Execute()
}

func handleSIGUSRx() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGUSR1)

	for range sig {
		pprof.Lookup("goroutine").WriteTo(os.Stderr, 2)
	}
}
