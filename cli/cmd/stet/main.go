package main

import "os"

func main() {
	os.Exit(Run())
}

// Run is the entry point for the CLI. It is exported for testing so that
// main.go can meet per-file coverage requirements.
func Run() int {
	return runCLI(os.Args[1:])
}

func runCLI(args []string) int {
	parseArgs(args)
	return 0
}

func parseArgs(args []string) {
	_ = args
}
