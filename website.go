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
	//port := ":27999"
	port := fmt.Sprintf(":%d", config.APIPort)

	fs := http.FileServer(http.Dir("assets/"))
	http.Handle("/assets/", http.StripPrefix("/assets/", fs))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		user := GetSessionUser(r)
		if user.ID != 0 {
			//fmt.Fprintf(w, "<p>User: %s</p>", user.UUID)
			http.Redirect(w, r, "/dashboard", http.StatusFound) // 302
			return
		}

		http.Redirect(w, r, "/login", http.StatusFound) // 302
	})

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		// login form submitted, process the login
		if r.Method == "POST" {
			if err := r.ParseForm(); err != nil {
				log.Println(err)
			}

			email := r.FormValue("email")
			//log.Printf("Submitted login address: %s\n", email)

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
	})

	http.HandleFunc("/dashboard/sv/", WebsiteServerHandler)
	http.HandleFunc("/dashboard", WebsiteDashboard)

	http.HandleFunc("/api/GetConnectedServers", func(w http.ResponseWriter, r *http.Request) {
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
	})

	log.Printf("Listening for web requests on %s\n", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func WebsiteDashboard(w http.ResponseWriter, r *http.Request) {

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

func WebsiteServerHandler(w http.ResponseWriter, r *http.Request) {
}
