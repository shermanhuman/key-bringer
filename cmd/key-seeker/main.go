package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if present
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		fmt.Println("Usage: key-seeker <command>")
		fmt.Println("Commands:")
		fmt.Println("  unlock --totp <code>  Unlock with TOTP code directly")
		fmt.Println("  --monitor             Wait for SMS approval and unlock")
		os.Exit(1)
	}

	// TODO: Implement CLI logic
	fmt.Println("key-seeker CLI not yet implemented")
}
