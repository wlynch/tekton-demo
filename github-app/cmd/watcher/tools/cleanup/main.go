package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/github"
)

func main() {
	// Wrap the shared transport for use with the app ID 1 authenticating with installation ID 99.
	keypath := filepath.Join(os.Getenv("HOME"), "Downloads", "wlynch-test.2020-07-09.private-key.pem")
	t, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, 9994, 111795, keypath)
	if err != nil {
		log.Fatal(err)
	}
	client := github.NewClient(&http.Client{Transport: t})

	client.Checks.
}
