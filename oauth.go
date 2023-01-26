package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
)

type Credentials struct {
	Type     string `json:"Type"`     // Identifier, "google", "discord"
	AuthURL  string `json:"AuthURL"`  // url to present login form
	TokenURL string `json:"TokenURL"` // url to fetch result token
	ClientID string `json:"ClientID"` // api "username"
	Secret   string `json:"Secret"`   // api "password"
	Icon     string `json:"Icon"`     // svg icon to display on website
	Alt      string `json:"Alt"`      // text to display on icon hover
	Enabled  bool   `json:"Enabled"`  // active or not
}

// Read the credentials file and load the contents.
// Line 1: should be the clientID
// Line 2: should be the client secret
func LoadOAuthCredentials(filename string) Credentials {
	contents, err := os.ReadFile(filename)
	if err != nil {
		log.Println("error reading oauth creds:", err)
		os.Exit(0)
	}

	lines := strings.Split(string(contents), "\n")
	if len(lines) < 2 {
		log.Println("credentials file", filename, "incomplete")
		os.Exit(0)
	}

	return Credentials{
		ClientID: lines[0],
		Secret:   lines[1],
	}
}

// Read json file holding our oauth2 providers.
// Called at webserver startup
func ReadOAuthCredsFromDisk(filename string) ([]Credentials, error) {
	cr := []Credentials{}
	filedata, err := os.ReadFile(filename)
	if err != nil {
		return cr, errors.New("unable to read credential file")
	}

	err = json.Unmarshal([]byte(filedata), &cr)
	if err != nil {
		return cr, errors.New("unable to parse credential data")
	}

	return cr, nil
}

// Write all credentials objects to json format on disk.
// Not sure this is really used for anything, but got for testing
func WriteOAuthCredsToDisk(creds []Credentials, filename string) error {
	filecontents, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, filecontents, 0644)
	if err != nil {
		return err
	}

	return nil
}
