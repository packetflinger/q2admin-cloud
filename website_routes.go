package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

type WebRoutes struct {
	Assets           string
	AuthLogin        string
	AuthLogout       string
	ConnectedServers string
	Dashboard        string
	Index            string
	ServerAdd        string
	ServerRemove     string
	ServerView       string
}

var routes WebRoutes

func LoadWebsiteRoutes() *mux.Router {
	routes.Assets = "/assets/"
	routes.AuthLogin = "/signin"
	routes.AuthLogout = "/signout"
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
	r.HandleFunc(routes.Dashboard, WebsiteHandlerDashboard)
	r.HandleFunc(routes.ServerRemove, WebDelServer)
	r.HandleFunc(routes.ServerView, WebsiteHandlerServerView)
	r.HandleFunc(routes.ConnectedServers, WebsiteAPIGetConnectedServers)
	r.PathPrefix(routes.Assets).Handler(http.FileServer(http.Dir(".")))

	return r
}