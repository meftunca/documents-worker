package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: documents-worker [server|cli] [args...]")
		fmt.Println("  server: Start the HTTP server")
		fmt.Println("  cli:    Run CLI commands")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "server":
		fmt.Println("🚀 Starting Documents Worker Server...")
		fmt.Println("Note: Use 'go run cmd/server/main.go' to start the server")
	case "cli":
		fmt.Println("🔧 Starting Documents Worker CLI...")
		fmt.Println("Note: Use 'go run cmd/cli/main.go [command]' to run CLI commands")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		fmt.Println("Available commands: server, cli")
		os.Exit(1)
	}
}
