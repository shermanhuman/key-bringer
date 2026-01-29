package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
)

func main() {
	name := flag.String("name", "KeyBringer", "Account name for the TOTP entry")
	issuer := flag.String("issuer", "KeyBringer", "Issuer name")
	output := flag.String("output", "", "Output QR code to PNG file (optional)")
	flag.Parse()

	// Generate a new TOTP key
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      *issuer,
		AccountName: *name,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate TOTP key: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== TOTP Secret Generated ===")
	fmt.Println()
	fmt.Printf("Secret (Base32): %s\n", key.Secret())
	fmt.Println()
	fmt.Println("Add to GCP Secret Manager:")
	fmt.Printf("  echo -n \"%s\" | gcloud secrets create totp-seed --data-file=-\n", key.Secret())
	fmt.Println()
	fmt.Printf("OTP Auth URL: %s\n", key.URL())
	fmt.Println()

	if *output != "" {
		// Generate QR code PNG
		err := qrcode.WriteFile(key.URL(), qrcode.Medium, 256, *output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write QR code: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("QR code saved to: %s\n", *output)
	} else {
		// Print QR code to terminal
		qr, err := qrcode.New(key.URL(), qrcode.Medium)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate QR code: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Scan this QR code with your authenticator app:")
		fmt.Println()
		fmt.Println(qr.ToSmallString(false))
	}
}
