package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/packetflinger/q2admind/util"
	"github.com/ravener/discord-oauth2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Credentials struct {
	Type        string `json:"Type"`        // Identifier, "google", "discord"
	AuthURL     string `json:"AuthURL"`     // url to present login form
	TokenURL    string `json:"TokenURL"`    // url to fetch result token
	ClientID    string `json:"ClientID"`    // api "username"
	Secret      string `json:"Secret"`      // api "password"
	Scope       string `json:"Scope"`       // what we want to access
	Icon        string `json:"Icon"`        // svg icon to display on website
	Alt         string `json:"Alt"`         // text to display on icon hover
	CallbackURL string `json:"CallbackURL"` // stage 2 url
	Enabled     bool   `json:"Enabled"`     // active or not
	URL         string // this is the "compiled" authurl
}

type AuthResponse struct {
	Token   string `json:"access_token"`
	Expires int    `json:"expires_in"`
	Type    string `json:"token_type"`
}

type ProfileResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Email    string `json:"email"`
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

// Construct a full auth url for oauth2 providers.
// Some realtime randomness is required, so it can't
// be pre-built
//
// Called from WebsiteHandlerSignin() for each provider
func BuildAuthURL(c Credentials, index int) string {
	state := fmt.Sprintf("%s|%d|%s", c.Type, index, util.GenerateUUID())
	url := fmt.Sprintf(
		"%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
		c.AuthURL,
		c.ClientID,
		url.QueryEscape(c.CallbackURL),
		url.QueryEscape(c.Scope),
		state,
	)
	return url
}

// Process the code returned from oauth provider after signin attemp.
//
// Called directly via web (/oauth-processor)
func ProcessOAuthReplyOld(w http.ResponseWriter, r *http.Request) {
	vars := r.URL.Query()
	code := vars.Get("code")
	state := vars.Get("state")

	parts := strings.Split(state, "|")
	if len(parts) != 3 {
		log.Println("auth fail: state returned in invalid format")
		return
	}

	index, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Println("auth fail: converting credential index from string to int")
		return
	}
	cred := Website.Creds[index]

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {cred.ClientID},
		"client_secret": {cred.Secret},
		"redirect_uri":  {cred.CallbackURL},
		"scope":         {cred.Scope},
		"code":          {code},
	}

	res, err := http.PostForm(cred.TokenURL, data)
	if err != nil {
		log.Println("auth fail:", err)
		return
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Println(string(body))

	response := AuthResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Println(err)
	}

	data = url.Values{
		"Authorization": {fmt.Sprintf("%s %s", response.Type, response.Token)},
	}

	profileres, err := http.PostForm("https://discord.com/api/users/@me", data)
	if err != nil {
		log.Println("profile lookup fail:", err)
		return
	}
	defer profileres.Body.Close()

	profilebody, err := ioutil.ReadAll(profileres.Body)
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Println(data)
	fmt.Println(string(profilebody))
}

// Someone authenticated via Discord and have been redirected to the
// callback url.
//
// Called just after signon at the oauth2 provider
func ProcessDiscordLogin(w http.ResponseWriter, r *http.Request) {
	vars := r.URL.Query()
	code := vars.Get("code")
	state := vars.Get("state")

	parts := strings.Split(state, "|")
	if len(parts) != 3 {
		log.Println("auth fail: state returned in invalid format")
		return
	}

	index, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Println("auth fail: converting credential index from string to int")
		return
	}
	cred := Website.Creds[index]

	conf := &oauth2.Config{
		RedirectURL:  cred.CallbackURL,
		ClientID:     cred.ClientID,
		ClientSecret: cred.Secret,
		Scopes:       []string{discord.ScopeIdentify},
		Endpoint:     discord.Endpoint,
	}

	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		log.Println(err)
		return
	}

	res, err := conf.Client(context.Background(), token).Get("https://discord.com/api/users/@me")
	if err != nil || res.StatusCode != 200 {
		log.Println(err)
		return
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println(err)
		return
	}

	profres := ProfileResponse{}
	err = json.Unmarshal(body, &profres)
	if err != nil {
		log.Println(err)
		return
	}

	//user, err := q2a.GetUserByEmail(profres.Email)
	if err != nil {
		log.Println(err)
	} else {
		/*
			user.Session = UserSession{
				ID:      token.AccessToken,
				Expires: token.Expiry.Unix(),
			}
			user.Avatar = fmt.Sprintf(
				"https://cdn.discordapp.com/avatars/%s/%s.png", profres.ID, profres.Avatar,
			)
		*/
		cookie := http.Cookie{
			Name:     SessionName,
			Value:    token.AccessToken,
			SameSite: http.SameSiteLaxMode,
			Expires:  token.Expiry,
			Path:     "/",
		}
		http.SetCookie(w, &cookie)
		http.Redirect(w, r, Routes.Dashboard, http.StatusFound) // 302
	}
}

func ProcessGoogleLogin(w http.ResponseWriter, r *http.Request) {
	oauthGoogleUrlAPI := "https://www.googleapis.com/oauth2/v2/userinfo?access_token="
	vars := r.URL.Query()
	code := vars.Get("code")
	state := vars.Get("state")

	parts := strings.Split(state, "|")
	if len(parts) != 3 {
		log.Println("auth fail: state returned in invalid format")
		return
	}

	index, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Println("auth fail: converting credential index from string to int")
		return
	}
	cred := Website.Creds[index]

	conf := &oauth2.Config{
		RedirectURL:  "http://localhost:8087/oauth-processor",
		ClientID:     cred.ClientID,
		ClientSecret: cred.Secret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}

	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Println(token)

	res, err := conf.Client(context.Background(), token).Get(oauthGoogleUrlAPI + token.AccessToken)
	if err != nil || res.StatusCode != 200 {
		log.Println(err)
		return
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Println(body)
	profres := ProfileResponse{}
	err = json.Unmarshal(body, &profres)
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println(profres)
}
