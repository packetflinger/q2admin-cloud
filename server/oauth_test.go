package server

import "testing"

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
