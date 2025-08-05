package backend

import (
	"net/http"

	"github.com/gorilla/mux"
)

type WebRoutes struct {
	Static           string
	Static2          string
	AuthLogin        string
	AuthLogout       string
	AuthGoogle       string
	AuthDiscord      string
	ConnectedServers string
	Dashboard        string
	Index            string
	Groups           string
	Privacy          string
	ServerAdd        string
	ServerRemove     string
	Servers          string
	ServerView       string
	Terms            string
}

type APIRoutes struct {
	MyServers     string
	ServerDetails string
	ServerList    string
	APIKeyList    string
}

var (
	Routes   WebRoutes
	apiRoute APIRoutes
)

func LoadWebsiteRoutes() *mux.Router {
	apiRoute.ServerList = "/api/v1/ListServers"
	apiRoute.APIKeyList = "/api/v1/ListAPIKeys/{UUID}/key/{APIKEY}"

	Routes.AuthDiscord = "/auth/discord"
	Routes.AuthGoogle = "/auth/google"
	Routes.AuthLogin = "/sign-in"
	Routes.AuthLogout = "/sign-out"
	Routes.ConnectedServers = "/api/GetConnectedServers"
	Routes.Dashboard = "/dashboard"
	Routes.Index = "/"
	Routes.Groups = "/my-groups"
	Routes.Privacy = "/privacy-policy"
	Routes.Static = "/static/"
	Routes.Static2 = "/static2/"
	Routes.ServerAdd = "/add-server"
	Routes.ServerRemove = "/dashboard/rm/{id}"
	Routes.Servers = "/my-servers"
	Routes.ServerView = "/sv/{ServerUUID}/{ServerName}"
	Routes.Terms = "/terms-of-use"

	r := mux.NewRouter()
	r.HandleFunc(Routes.Index, WebsiteHandlerIndex)
	r.HandleFunc(Routes.ServerAdd, WebAddServer).Methods("POST")
	r.HandleFunc(Routes.AuthLogin, WebsiteHandlerSignin)
	r.HandleFunc(Routes.AuthLogout, WebSignout)
	r.HandleFunc(Routes.AuthDiscord, ProcessDiscordLogin)
	r.HandleFunc(Routes.AuthGoogle, ProcessGoogleLogin)
	r.HandleFunc(Routes.Dashboard, WebsiteHandlerDashboard)
	r.HandleFunc(Routes.ServerRemove, WebDelServer)
	r.HandleFunc(Routes.ServerView, WebsiteHandlerServerView)
	r.HandleFunc(Routes.ConnectedServers, WebsiteAPIGetConnectedServers)
	r.HandleFunc(Routes.Groups, GroupsHandler)
	r.HandleFunc(Routes.Privacy, PrivacyHandler)
	r.HandleFunc(Routes.Servers, ServersHandler)
	r.HandleFunc(Routes.Terms, TermsHandler)

	r.PathPrefix(Routes.Static).Handler(http.FileServer(http.Dir("./api/website")))
	r.PathPrefix(Routes.Static2).Handler(http.FileServer(http.Dir("./api/website")))

	r.HandleFunc(apiRoute.ServerList, APIServerList)
	r.HandleFunc(apiRoute.APIKeyList, APIKeyList)

	return r
}
