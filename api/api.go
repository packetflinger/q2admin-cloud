package api

type APIReply struct {
	Status       int    // http code (404, 500, 508, etc)
	StatusString string // string explaining the code
}
type APIServersReply struct {
	Response APIReply // successful or not?
	Count    int      // how many servers we have
	//Servers  []cloud Client // the server struct
}

/*
// APIGetMyServers will send an http response containing a JSON
// structure of all of "my" servers. The "my" part determined by
// session.
//
// Unlike normal web handlers, if session isn't valid, don't
// redirect to a signin page, just send an error.
func APIGetMyServers(w http.ResponseWriter, r *http.Request) {
	res := APIServersReply{}
	user, err := GetSessionUser(r)
	if err != nil {
		res.Response.Status = http.StatusForbidden
		res.Response.StatusString = "invalid session"
		contents, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Fprintln(w, string(contents))
		return
	}

	svs := []client.Client{}
	for _, sv := range .Q2A.clients {
		if sv.Owner == user.ID {
			svs = append(svs, sv)
		}
	}

	res.Response.Status = http.StatusOK
	res.Response.StatusString = "ok"
	res.Servers = svs
	res.Count = len(svs)

	contents, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		log.Println(err)
		return
	}

	_, err = fmt.Fprintln(w, string(contents))
	if err != nil {
		log.Println(err)
	}
}
*/
