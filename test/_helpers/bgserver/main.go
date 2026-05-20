package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	fifoPath := flag.String("fifo", "", "path to named pipe for receiving messages")
	flag.Parse()

	// Create FIFO if path provided
	if *fifoPath != "" {
		os.Remove(*fifoPath) // clean up any stale file
		if err := syscall.Mkfifo(*fifoPath, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "mkfifo: %v\n", err)
			os.Exit(1)
		}
		defer os.Remove(*fifoPath)
	}

	fmt.Println("started")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)

	// Read messages from FIFO in background goroutine
	if *fifoPath != "" {
		go func() {
			f, err := os.Open(*fifoPath)
			if err != nil {
				return
			}
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				fmt.Printf("msg: %s\n", scanner.Text())
			}
		}()
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Println("still running")
		case <-sig:
			fmt.Fprintln(os.Stderr, "graceful termination")
			os.Exit(0)
		}
	}
}
