package backend

import (
	"errors"
	"net/http"

	"github.com/packetflinger/q2admind/frontend"
	pb "github.com/packetflinger/q2admind/proto"
)

// who is allowed to access what
type IdentityAccess struct {
	email     string
	frontends []string
}

type IdentityContext struct {
	user      *pb.User
	apiKey    string
	frontends []*frontend.Frontend
	srcIP     string
}

// CreateIdentContext will make a context based on the input request. This will
// check that there is either an active session (cookie-based) or an API key
// was used. The context includes pointers to clients it has access to.
//
// If both are present, session wins
func CreateIdentContext(request *http.Request) (*IdentityContext, error) {
	ctx := IdentityContext{}
	ctx.srcIP = request.RemoteAddr
	// first check for active session
	if cookie, err := request.Cookie(CookieName); err == nil {
		user, err := ValidateSession(cookie.Value)
		if err != nil {
			return nil, err
		}
		ctx.user = user
	}

	// then look for a "key" in the query string
	if apiKey := request.URL.Query().Get("key"); len(apiKey) > 0 {
		ctx.apiKey = apiKey
	}

	ctx.frontends = FrontendsByContext(&ctx)
	return &ctx, nil
}

// Is the supplied identity allowed to access this client identified by uuid?
// If so, return a pointer to that client
func identityAllowed(ident *IdentityContext, uuid string) (*frontend.Frontend, error) {
	fe, err := be.FindFrontend(uuid)
	if err != nil {
		return nil, err
	}

	if ident.user.Email == fe.Owner {
		return fe, nil
	}

	if len(ident.apiKey) > 0 {
		for _, k := range fe.APIKeys.GetKey() {
			if k.GetSecret() == ident.apiKey {
				return fe, nil
			}
		}
	}
	return nil, errors.New("not allowed")
}
