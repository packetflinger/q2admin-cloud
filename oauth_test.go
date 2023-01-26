package main

import "testing"

func TestReadCreds(t *testing.T) {
	c, err := ReadOAuthCredsFromDisk("oauthcreds.json")
	if err != nil {
		t.Error(err)
	}

	if c[0].Type != "google" {
		t.Error("first one isn't google")
	}

	if c[1].Type != "discord" {
		t.Error("second one isn't discord")
	}
}

func TestWriteCreds(t *testing.T) {
	c := []Credentials{
		{
			Type:     "google",
			AuthURL:  "lkjasf",
			TokenURL: "lkjasf",
			ClientID: "lkjasf",
			Secret:   "lkjasf",
			Icon:     "google-icon.svg",
			Alt:      "Login with your Google account",
			Enabled:  true,
		},
		{
			Type:     "discord",
			AuthURL:  "lkjasf",
			TokenURL: "lkjasf",
			ClientID: "lkjasf",
			Secret:   "lkjasf",
			Icon:     "discord-icon.svg",
			Alt:      "Login with your Discord account",
			Enabled:  true,
		},
	}

	err := WriteOAuthCredsToDisk(c, "oauthcreds.json")
	if err != nil {
		t.Error(err)
	}
}
