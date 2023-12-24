package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type ServerWithUUID struct {
	Name string
	UUID string
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
