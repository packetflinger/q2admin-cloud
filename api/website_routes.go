package main

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
	routes WebRoutes
	api    APIRoutes
)

func LoadWebsiteRoutes() *mux.Router {
	api.MyServers = "/api/v1/GetMyServers"

	routes.Static = "/static/"
	routes.Static2 = "/static2/"
	routes.AuthLogin = "/signin"
	routes.AuthLogout = "/signout"
	routes.AuthGoogle = "/auth/google"
	routes.AuthDiscord = "/auth/discord"
	routes.ChatFeed = "/dashboard/sv/{ServerUUID}/feed"
	routes.ChatFeedInput = "/dashboard/sv/{ServerUUID}/input"
	routes.ConnectedServers = "/api/GetConnectedServers"
	routes.Dashboard = "/dashboard"
	routes.Index = "/"
	routes.Groups = "/my-groups"
	routes.Privacy = "/privacy-policy"
	routes.ServerAdd = "/add-server"
	routes.ServerRemove = "/dashboard/rm/{id}"
	routes.Servers = "/my-servers"
	routes.ServerView = routes.Servers + "/{ServerUUID}/{ServerName}"
	routes.Terms = "/terms-of-use"

	r := mux.NewRouter()
	r.HandleFunc(routes.Index, WebsiteHandlerIndex)
	r.HandleFunc(routes.ServerAdd, WebAddServer).Methods("POST")
	r.HandleFunc(routes.AuthLogin, WebsiteHandlerSignin)
	r.HandleFunc(routes.AuthLogout, WebSignout)
	r.HandleFunc(routes.AuthDiscord, ProcessDiscordLogin)
	r.HandleFunc(routes.AuthGoogle, ProcessGoogleLogin)
	r.HandleFunc(routes.ChatFeed, WebFeed)
	r.HandleFunc(routes.ChatFeedInput, WebFeedInput)
	r.HandleFunc(routes.Dashboard, WebsiteHandlerDashboard)
	r.HandleFunc(routes.ServerRemove, WebDelServer)
	r.HandleFunc(routes.ServerView, WebsiteHandlerServerView)
	r.HandleFunc(routes.ConnectedServers, WebsiteAPIGetConnectedServers)
	r.HandleFunc(routes.Groups, GroupsHandler)
	r.HandleFunc(routes.Privacy, PrivacyHandler)
	r.HandleFunc(routes.Servers, ServersHandler)
	r.HandleFunc(routes.Terms, TermsHandler)

	r.PathPrefix(routes.Static).Handler(http.FileServer(http.Dir("./website")))
	r.PathPrefix(routes.Static2).Handler(http.FileServer(http.Dir("./website")))

	r.HandleFunc(api.MyServers, APIGetMyServers)

	return r
}
