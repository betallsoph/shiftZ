// Command hashpassword reads a password from stdin and prints a bcrypt hash.
// Use for generating ADMIN_PASSWORD_HASH; never log the plaintext password.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	password, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hashpassword: read stdin:", err)
		os.Exit(1)
	}
	password = bytes.TrimSuffix(password, []byte("\n"))
	password = bytes.TrimSuffix(password, []byte("\r"))
	if len(password) == 0 {
		fmt.Fprintln(os.Stderr, "hashpassword: password must not be empty")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hashpassword:", err)
		os.Exit(1)
	}
	fmt.Print(string(hash))
}
