package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	pb "github.com/packetflinger/q2admind/proto"
)

type ServerWithUUID struct {
	Name string
	UUID string
}

type APIKeyListResponse struct {
	State  APIResponse
	Server ServerWithUUID
	Keys   *pb.ApiKeys
}

// generic, included in every response
type APIResponse struct {
	Code        int    // standard http code: 200 = good
	Description string // what happend?
}
type APIServeListResponse struct {
	State   APIResponse
	Servers []ServerWithUUID
}

// APIKeyList will fetch an array of active API keys for the client
func APIKeyList(w http.ResponseWriter, r *http.Request) {
	res := APIKeyListResponse{}
	ident, err := CreateIdentContext(r)
	fmt.Println(ident)
	if err != nil {
		res.State = APIResponse{
			Code:        http.StatusForbidden,
			Description: "invalid session",
		}
		contents, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Fprintln(w, string(contents))
		return
	}

	// uuid in the url, this is the client we want to access
	uuid := mux.Vars(r)["UUID"]
	client, err := identityAllowed(ident, uuid)
	if err != nil {
		log.Println("APIKeyList:", err)
		return
	}
	res.Keys = client.APIKeys
	res.State = APIResponse{
		Code:        http.StatusOK,
		Description: "ok",
	}
	res.Server = ServerWithUUID{
		Name: client.Name,
		UUID: client.UUID,
	}
	contents, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Fprintln(w, string(contents))
}

// APIServerList generates a JSON object containing all servers
// linked to the session user.
func APIServerList(w http.ResponseWriter, r *http.Request) {
	res := APIServeListResponse{}
	user, err := GetSessionUser(r)
	if err != nil {
		res.State.Code = http.StatusForbidden
		res.State.Description = "invalid session"
		contents, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Fprintln(w, string(contents))
		return
	}

	clientList := []ServerWithUUID{}
	cls := ClientsByIdentity(user.Email)
	for _, c := range cls {
		clientList = append(clientList, ServerWithUUID{
			Name: c.Name,
			UUID: c.UUID,
		})
	}
	res.Servers = clientList
	res.State = APIResponse{
		Code:        http.StatusOK,
		Description: "ok",
	}
	contents, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Fprintln(w, string(contents))
}
