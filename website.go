package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type ActiveServer struct {
	UUID        string
	Name        string
	Playercount int
}

type WebUser struct {
	ID          int
	UUID        string
	Email       string
	ServerCount int
	Admin       bool
}

type DashboardPage struct {
	User         WebUser
	MyServers    []Server
	OtherServers []Server
}

/**
 * Checks if user has an existing session and validates it
 */
func GetSessionUser(r *http.Request) WebUser {
	var userid int
	var cookie *http.Cookie
	var e error
	niluser := &WebUser{}

	if cookie, e = r.Cookie(SessionName); e != nil {
		return *niluser
	}

	if userid, e = ValidateSession(cookie.Value); e != nil {
		return *niluser
	}

	return GetUser(userid)
}

func GetUser(id int) WebUser {
	niluser := &WebUser{}
	sql := "SELECT id, uuid, email, server_count, admin FROM user WHERE id = ? LIMIT 1"
	r, e := db.Query(sql, id)
	if e != nil {
		log.Println(e)
		return *niluser
	}

	var user WebUser
	for r.Next() {
		r.Scan(&user.ID, &user.UUID, &user.Email, &user.ServerCount, &user.Admin)
		r.Close()
		return user
	}

	return *niluser
}

/**
 * User just successfully authed, insert a new session
 */
func CreateSession(user int) string {
	sessionid := uuid.New().String()
	expires := GetUnixTimestamp() + (86400 * 2) // two day from now

	sql := "INSERT INTO websession (session, user, expiration) VALUES (?, ?, ?)"
	_, err := db.Exec(sql, sessionid, user, expires)
	if err != nil {
		log.Println(err)
	}
	//r.Close()

	sql = "UPDATE user SET last_login = ? WHERE id = ? LIMIT 1"
	_, err = db.Exec(sql, GetUnixTimestamp(), user)
	if err != nil {
		log.Println(err)
	}
	//r.Close()

	return sessionid
}

func ValidateSession(sess string) (int, error) {
	var UserID int
	sql := "SELECT user FROM websession WHERE session = ? AND expiration >= ? LIMIT 1"
	if r, e := db.Query(sql, sess, GetUnixTimestamp()); e == nil {
		r.Next()
		r.Scan(&UserID)
		r.Close()
		return UserID, nil
	} else {
		return 0, errors.New(e.Error())
	}
}

func RunHTTPServer() {
	port := fmt.Sprintf(":%d", config.APIPort)

	r := mux.NewRouter()
	r.HandleFunc("/", WebsiteHandlerIndex)
	r.HandleFunc("/signin", WebsiteHandlerSignin)
	r.HandleFunc("/dashboard", WebsiteHandlerDashboard)
	r.HandleFunc("/dashboard/sv/{ServerUUID}", WebsiteHandlerServerView)
	r.HandleFunc("/api/GetConnectedServers", WebsiteAPIGetConnectedServers)
	r.PathPrefix("/assets/").Handler(http.FileServer(http.Dir(".")))

	log.Printf("Listening for web requests on %s\n", port)

	httpsrv := &http.Server{
		Handler:      r,
		Addr:         port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(httpsrv.ListenAndServe())
}

func WebsiteHandlerDashboard(w http.ResponseWriter, r *http.Request) {

	page := DashboardPage{}

	page.User = GetSessionUser(r)
	if page.User.ID == 0 {
		http.Redirect(w, r, "/login", http.StatusFound) // 302
	}

	for _, sv := range servers {
		if sv.Owner == page.User.ID {
			page.MyServers = append(page.MyServers, sv)
		}
	}

	tmpl, e := template.ParseFiles("website-templates/dashboard.tmpl")
	if e != nil {
		log.Println(e)
	} else {
		tmpl.Execute(w, page)
	}
}

func WebsiteHandlerServerView(w http.ResponseWriter, r *http.Request) {
}

//
// the "index" handler
//
func WebsiteHandlerIndex(w http.ResponseWriter, r *http.Request) {
	user := GetSessionUser(r)
	if user.ID != 0 {
		http.Redirect(w, r, "/dashboard", http.StatusFound) // 302
		return
	}

	http.Redirect(w, r, "/signin", http.StatusFound) // 302
}

//
// Handle logins
//
func WebsiteHandlerSignin(w http.ResponseWriter, r *http.Request) {
	// login form submitted, process the login
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			log.Println(err)
		}

		email := r.FormValue("email")

		// lookup the user's ID
		var UserID int
		sql := "SELECT id FROM user WHERE email = ? LIMIT 1"
		rs, e := db.Query(sql, email)
		if e == nil {
			rs.Next()
			rs.Scan(&UserID)
			rs.Close()
			//log.Printf("Userid lookup: %d\n", UserID)
			sess := CreateSession(UserID)
			cookieexpire := time.Now().AddDate(0, 0, 2) // years, months, days
			cookie := http.Cookie{Name: SessionName, Value: sess, Expires: cookieexpire}
			http.SetCookie(w, &cookie)
		}

		http.Redirect(w, r, "/dashboard", http.StatusFound) // 302
		return
	}

	// ...or show the form
	tmpl, e := template.ParseFiles("website-templates/login.tmpl")
	if e != nil {
		log.Println(e)
	} else {
		tmpl.Execute(w, nil)
	}
}

func WebsiteAPIGetConnectedServers(w http.ResponseWriter, r *http.Request) {
	var activeservers []ActiveServer
	for _, s := range servers {
		if s.connected {
			srv := ActiveServer{UUID: s.UUID, Name: s.Name, Playercount: len(s.players)}
			activeservers = append(activeservers, srv)
		}
	}

	j, e := json.Marshal(activeservers)
	if e != nil {
		fmt.Println(e)
		fmt.Fprintf(w, "{}")
		return
	}

	fmt.Fprintf(w, string(j))
}
