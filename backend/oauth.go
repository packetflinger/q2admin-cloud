package backend

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

	"github.com/google/uuid"
	pb "github.com/packetflinger/q2admind/proto"
	"github.com/packetflinger/q2admind/util"
	"github.com/ravener/discord-oauth2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/protobuf/encoding/prototext"
)

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

type GoogleProfileResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
	HD            string `json:"hd"`
}

// Read json file holding our oauth2 providers.
// Called at webserver startup
func ReadOAuthCredsFromDisk(filename string) ([]*pb.OAuth, error) {
	cr := pb.Credentials{}
	filedata, err := os.ReadFile(filename)
	if err != nil {
		return []*pb.OAuth{}, errors.New("unable to read credential file")
	}

	err = prototext.Unmarshal(filedata, &cr)
	if err != nil {
		return []*pb.OAuth{}, errors.New("unable to parse credential data")
	}

	return cr.GetOauth(), nil
}

// Write all credentials objects to json format on disk.
// Not sure this is really used for anything, but got for testing
func WriteOAuthCredsToDisk(creds []*pb.OAuth, filename string) error {
	cr := pb.Credentials{
		Oauth: creds,
	}
	filecontents, err := prototext.Marshal(&cr)
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
func BuildAuthURL(cred *pb.OAuth, index int) string {
	state := fmt.Sprintf("%s|%d|%s", cred.GetType().String(), index, util.GenerateUUID())
	url := fmt.Sprintf(
		"%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
		cred.GetAuthUrl(),
		cred.GetClientId(),
		url.QueryEscape(cred.GetCallbackUrl()),
		url.QueryEscape(strings.Join(cred.GetScope(), " ")),
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
		"client_id":     {cred.GetClientId()},
		"client_secret": {cred.GetSecret()},
		"redirect_uri":  {cred.GetCallbackUrl()},
		"scope":         {strings.Join(cred.GetScope(), ",")},
		"code":          {code},
	}

	res, err := http.PostForm(cred.GetTokenUrl(), data)
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

	tokens := strings.Split(state, "|")
	if len(tokens) != 3 {
		log.Println("auth fail: state returned in invalid format")
		return
	}

	index, err := strconv.Atoi(tokens[1])
	if err != nil {
		log.Println("auth fail: converting credential index from string to int")
		return
	}
	cred := Website.Creds[index]

	conf := &oauth2.Config{
		RedirectURL:  cred.GetCallbackUrl(),
		ClientID:     cred.GetClientId(),
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

	user, err := be.GetUserByEmail(profres.Email)
	if err != nil {
		log.Println(err)
	} else {
		alias := profres.Username
		if len(user.Name) > 0 {
			alias = user.Name
		}
		pic := fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", profres.ID, profres.Avatar)
		session := pb.Session{
			Id:         uuid.New().String(),
			Creation:   util.GetUnixTimestamp(),
			Expiration: token.Expiry.Unix(),
			AuthToken:  token.AccessToken,
			Avatar:     pic,
			Name:       alias,
		}
		user.Session = &session

		cookie := http.Cookie{
			Name:     SessionName,
			Value:    session.GetId(),
			SameSite: http.SameSiteLaxMode,
			Expires:  token.Expiry,
			Path:     "/",
		}
		http.SetCookie(w, &cookie)
		http.Redirect(w, r, Routes.Dashboard, http.StatusFound) // 302
	}
}

// TODO: hard code less of this
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
		RedirectURL:  cred.GetCallbackUrl(),
		ClientID:     cred.GetClientId(),
		ClientSecret: cred.Secret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
			"openid",
		},
		Endpoint: google.Endpoint,
	}

	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		log.Println(err)
		return
	}

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

	profres := GoogleProfileResponse{}
	err = json.Unmarshal(body, &profres)
	if err != nil {
		log.Println(err)
		return
	}

	user, err := be.GetUserByEmail(profres.Email)
	if err != nil {
		log.Println(err)
	} else {
		alias := profres.GivenName
		if len(user.Name) > 0 {
			alias = user.Name
		}
		session := pb.Session{
			Id:         uuid.New().String(),
			Creation:   util.GetUnixTimestamp(),
			Expiration: token.Expiry.Unix(),
			AuthToken:  token.AccessToken,
			Avatar:     profres.Picture,
			Name:       alias,
		}
		user.Session = &session

		cookie := http.Cookie{
			Name:     SessionName,
			Value:    session.GetId(),
			SameSite: http.SameSiteLaxMode,
			Expires:  token.Expiry,
			Path:     "/",
		}
		http.SetCookie(w, &cookie)
		http.Redirect(w, r, Routes.Dashboard, http.StatusFound) // 302
	}
}
