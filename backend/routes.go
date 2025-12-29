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
	ServerEdit       string
	ServerChangeUUID string
	Terms            string
	PlayerView       string
	ServerConsole    string
	Search           string
	SearchServer     string
	PlayerSearchView string
	RuleList         string
	RuleView         string
	RuleEdit         string
	ServerKeys       string
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

	Routes.ServerAdd = "/add-server"
	Routes.ConnectedServers = "/api/GetConnectedServers"
	Routes.AuthDiscord = "/auth/discord"
	Routes.AuthGoogle = "/auth/google"
	Routes.ServerRemove = "/dashboard/rm/{id}"
	Routes.Dashboard = "/dashboard"
	Routes.Groups = "/my-groups"
	Routes.Servers = "/my-servers"
	Routes.PlayerSearchView = "/player/{lookup}"
	Routes.Privacy = "/privacy-policy"
	Routes.RuleView = "/rules/{uuid}/view"
	Routes.RuleEdit = "/rules/{uuid}/edit"
	Routes.SearchServer = "/search/{UUID}"
	Routes.Search = "/search"
	Routes.AuthLogin = "/sign-in"
	Routes.AuthLogout = "/sign-out"
	Routes.Static = "/static/"
	Routes.Static2 = "/static2/"
	Routes.PlayerView = "/sv/{ServerUUID}/{ServerName}/player/{ClientNum}"
	Routes.ServerConsole = "/sv/{ServerUUID}/{ServerName}/console"
	Routes.ServerEdit = Routes.ServerView + "/edit"
	Routes.ServerChangeUUID = Routes.ServerView + "/changeuuid"
	Routes.ServerView = "/sv/{ServerUUID}/{ServerName}"
	Routes.ServerKeys = "/sv/{uuid}/keygen"
	Routes.RuleList = "/sv/{ServerUUID}/rules"
	Routes.Terms = "/terms-of-use"
	Routes.Index = "/"

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
	r.HandleFunc(Routes.ServerEdit, WebEditServer).Methods("POST")
	r.HandleFunc(Routes.ServerChangeUUID, ChangeUUIDHandler)
	r.HandleFunc(Routes.PlayerView, PlayerViewHandler)
	r.HandleFunc(Routes.ServerConsole, ServerConsoleHandler)
	r.HandleFunc(Routes.Search, SearchHandler)
	r.HandleFunc(Routes.SearchServer, SearchHandler)
	r.HandleFunc(Routes.PlayerSearchView, PlayerSearchViewHandler)
	r.HandleFunc(Routes.RuleView, RuleViewHandler)
	r.HandleFunc(Routes.RuleEdit, RuleEditHandler)
	r.HandleFunc(Routes.RuleList, RuleListHandler)
	r.HandleFunc(Routes.ServerKeys, ServerKeysHandler)

	r.PathPrefix(Routes.Static).Handler(http.FileServer(http.Dir("./api/website")))
	r.PathPrefix(Routes.Static2).Handler(http.FileServer(http.Dir("./api/website")))

	r.HandleFunc(apiRoute.ServerList, APIServerList)
	r.HandleFunc(apiRoute.APIKeyList, APIKeyList)

	return r
}
