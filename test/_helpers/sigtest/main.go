// sigtest is a test helper that handles signals with configurable behavior.
// Usage: sigtest [--exit N] [--ignore] [--stderr MSG]
//   --exit N     exit with code N on SIGTERM (default: 0)
//   --ignore     ignore SIGTERM entirely
//   --stderr MSG write MSG to stderr on SIGTERM before exiting
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	exitCode := flag.Int("exit", 0, "exit code on SIGTERM")
	ignore := flag.Bool("ignore", false, "ignore SIGTERM")
	stderrMsg := flag.String("stderr", "", "message to write to stderr on SIGTERM")
	flag.Parse()

	if *ignore {
		signal.Ignore(syscall.SIGTERM)
		fmt.Println("ready")
		select {} // block forever
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)

	fmt.Println("ready")

	<-sig
	if *stderrMsg != "" {
		fmt.Fprintln(os.Stderr, *stderrMsg)
	}
	os.Exit(*exitCode)
}
