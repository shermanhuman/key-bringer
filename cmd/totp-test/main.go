package main

import (
	"fmt"
	"time"

	"github.com/pquerna/otp/totp"
)

func main() {
	seed := "N3RDSOF2B5WL7Z4TXQIEGKUCPY6MVAJH"

	// Generate current code
	code, err := totp.GenerateCode(seed, time.Now())
	if err != nil {
		fmt.Printf("Error generating code: %v\n", err)
		return
	}

	fmt.Printf("Current time: %s\n", time.Now().Format(time.RFC3339))
	fmt.Printf("TOTP code for seed %s: %s\n", seed, code)

	// Validate it
	valid := totp.Validate(code, seed)
	fmt.Printf("Validates: %v\n", valid)
}
