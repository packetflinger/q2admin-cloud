package server

import (
	"errors"
	"net/http"

	"github.com/packetflinger/q2admind/client"
	pb "github.com/packetflinger/q2admind/proto"
)

type IdentityContext struct {
	user    *pb.User
	apiKey  string
	clients []*client.Client
	srcIP   string
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
	if cookie, err := request.Cookie(SessionName); err == nil {
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

	ctx.clients = ClientsByContext(&ctx)
	return &ctx, nil
}

// Is the supplied identity allowed to access this client identified by uuid?
// If so, return a pointer to that client
func identityAllowed(ident *IdentityContext, uuid string) (*client.Client, error) {
	cl, err := FindClient(uuid)
	if err != nil {
		return nil, err
	}

	if ident.user.Email == cl.Owner {
		return cl, nil
	}

	if len(ident.apiKey) > 0 {
		for _, k := range cl.APIKeys.GetKey() {
			if k.GetSecret() == ident.apiKey {
				return cl, nil
			}
		}
	}
	return nil, errors.New("not allowed")
}
