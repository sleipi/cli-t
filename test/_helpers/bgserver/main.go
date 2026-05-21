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
	envName := flag.String("env", "", "print the value of this environment variable on startup")
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

	// Open FIFO before printing "started" so it's ready for writers.
	// O_RDWR on a named pipe doesn't block (unlike O_RDONLY which waits for a writer).
	var fifoFile *os.File
	if *fifoPath != "" {
		var err error
		fifoFile, err = os.OpenFile(*fifoPath, os.O_RDWR, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open fifo: %v\n", err)
			os.Exit(1)
		}
		defer fifoFile.Close()
	}

	fmt.Println("started")

	if *envName != "" {
		fmt.Printf("env: %s=%s\n", *envName, os.Getenv(*envName))
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)

	// Read messages from FIFO in background goroutine
	if fifoFile != nil {
		go func() {
			scanner := bufio.NewScanner(fifoFile)
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
