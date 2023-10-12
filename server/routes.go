package server

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
	ChatFeed         string
	ChatFeedInput    string
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
	MyServers string
}

var (
	Routes   WebRoutes
	apiRoute APIRoutes
)

func LoadWebsiteRoutes() *mux.Router {
	apiRoute.MyServers = "/api/v1/GetMyServers"

	Routes.Static = "/static/"
	Routes.Static2 = "/static2/"
	Routes.AuthLogin = "/signin"
	Routes.AuthLogout = "/signout"
	Routes.AuthGoogle = "/auth/google"
	Routes.AuthDiscord = "/auth/discord"
	Routes.ChatFeed = "/dashboard/sv/{ServerUUID}/feed"
	Routes.ChatFeedInput = "/dashboard/sv/{ServerUUID}/input"
	Routes.ConnectedServers = "/api/GetConnectedServers"
	Routes.Dashboard = "/dashboard"
	Routes.Index = "/"
	Routes.Groups = "/my-groups"
	Routes.Privacy = "/privacy-policy"
	Routes.ServerAdd = "/add-server"
	Routes.ServerRemove = "/dashboard/rm/{id}"
	Routes.Servers = "/my-servers"
	Routes.ServerView = Routes.Servers + "/{ServerUUID}/{ServerName}"
	Routes.Terms = "/terms-of-use"

	r := mux.NewRouter()
	r.HandleFunc(Routes.Index, WebsiteHandlerIndex)
	r.HandleFunc(Routes.ServerAdd, WebAddServer).Methods("POST")
	r.HandleFunc(Routes.AuthLogin, WebsiteHandlerSignin)
	r.HandleFunc(Routes.AuthLogout, WebSignout)
	//r.HandleFunc(Routes.AuthDiscord, ProcessDiscordLogin)
	//r.HandleFunc(Routes.AuthGoogle, ProcessGoogleLogin)
	r.HandleFunc(Routes.ChatFeed, WebFeed)
	r.HandleFunc(Routes.ChatFeedInput, WebFeedInput)
	r.HandleFunc(Routes.Dashboard, WebsiteHandlerDashboard)
	r.HandleFunc(Routes.ServerRemove, WebDelServer)
	r.HandleFunc(Routes.ServerView, WebsiteHandlerServerView)
	r.HandleFunc(Routes.ConnectedServers, WebsiteAPIGetConnectedServers)
	r.HandleFunc(Routes.Groups, GroupsHandler)
	r.HandleFunc(Routes.Privacy, PrivacyHandler)
	r.HandleFunc(Routes.Servers, ServersHandler)
	r.HandleFunc(Routes.Terms, TermsHandler)

	r.PathPrefix(Routes.Static).Handler(http.FileServer(http.Dir("./website")))
	r.PathPrefix(Routes.Static2).Handler(http.FileServer(http.Dir("./website")))

	//r.HandleFunc(api.MyServers, APIGetMyServers)

	return r
}
