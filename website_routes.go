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
	ChatFeed         string
	ChatFeedInput    string
	ConnectedServers string
	Dashboard        string
	Index            string
	ServerAdd        string
	ServerRemove     string
	ServerView       string
}

var routes WebRoutes

func LoadWebsiteRoutes() *mux.Router {
	routes.Static = "/static/"
	routes.Static2 = "/static2/"
	routes.AuthLogin = "/signin"
	routes.AuthLogout = "/signout"
	routes.ChatFeed = "/dashboard/sv/{ServerUUID}/feed"
	routes.ChatFeedInput = "/dashboard/sv/{ServerUUID}/input"
	routes.ConnectedServers = "/api/GetConnectedServers"
	routes.Dashboard = "/dashboard"
	routes.Index = "/"
	routes.ServerAdd = "/add-server"
	routes.ServerRemove = "/dashboard/rm/{id}"
	routes.ServerView = "/dashboard/sv/{ServerUUID}"

	r := mux.NewRouter()
	r.HandleFunc(routes.Index, WebsiteHandlerIndex)
	r.HandleFunc(routes.ServerAdd, WebAddServer).Methods("POST")
	r.HandleFunc(routes.AuthLogin, WebsiteHandlerSignin)
	r.HandleFunc(routes.AuthLogout, WebSignout)
	r.HandleFunc(routes.ChatFeed, WebFeed)
	r.HandleFunc(routes.ChatFeedInput, WebFeedInput)
	r.HandleFunc(routes.Dashboard, WebsiteHandlerDashboard)
	r.HandleFunc(routes.ServerRemove, WebDelServer)
	r.HandleFunc(routes.ServerView, WebsiteHandlerServerView)
	r.HandleFunc(routes.ConnectedServers, WebsiteAPIGetConnectedServers)
	r.PathPrefix(routes.Static).Handler(http.FileServer(http.Dir("./website")))
	r.PathPrefix(routes.Static2).Handler(http.FileServer(http.Dir("./website")))

	return r
}
