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
    sql := "SELECT id, uuid, email, servercount, admin FROM user WHERE id = ? LIMIT 1"
    r, e := db.Query(sql, id)
    if e != nil {
        log.Println(e)
        return *niluser
    }

    var user WebUser
    for r.Next() {
        r.Scan(&user.ID, &user.UUID, &user.Email, &user.ServerCount, &user.Admin)
        return user
    }

    return *niluser
}

/**
 * User just successfully authed, remove any existing sessions and make
 * a new one
 */
func CreateSession(user int) string {
    sessionid := uuid.New().String()

    // remove any old sessions
    sql := "DELETE FROM websession WHERE user = ?"
    _, err := db.Query(sql, user)
    if err != nil {
        log.Println(err)
    }

    // add the new session
    sql = "INSERT INTO websession (session, user, expiration) VALUES (?, ?, NOW() + INTERVAL 2 DAY)"
    _, err = db.Query(sql, sessionid, user)
    if err != nil {
        log.Println(err)
    }

    return sessionid
}

func ValidateSession(sess string) (int, error) {
    var UserID int
    sql := "SELECT user FROM websession WHERE session = ? AND expiration >= NOW() LIMIT 1"
    if r, e := db.Query(sql, sess); e == nil {
        r.Next()
        r.Scan(&UserID)
        return UserID, nil
    } else {
        return 0, errors.New(e.Error())
    }
}

func RunHTTPServer() {
    port := ":27999"

    fs := http.FileServer(http.Dir("assets/"))
    http.Handle("/assets/", http.StripPrefix("/assets/", fs))

    http.HandleFunc("/", func (w http.ResponseWriter, r *http.Request) {
        user := GetSessionUser(r)
        if user.ID != 0 {
            fmt.Fprintf(w, "<p>User: %s</p>", user.UUID)
            return
        }

        http.Redirect(w, r, "/login", http.StatusFound) // 302
    })

    http.HandleFunc("/login", func (w http.ResponseWriter, r *http.Request) {
        // login form submitted, process the login
        if r.Method == "POST" {
            if err := r.ParseForm(); err != nil {
                log.Println(err)
            }

            email := r.FormValue("email")

            // lookup the user's ID
            var UserID int
            sql := "SELECT id FROM user WHERE email = ? LIMIT 1"
            r, e := db.Query(sql, email)
            if e == nil {
                r.Next()
                r.Scan(&UserID)

                sess := CreateSession(UserID)
                cookieexpire := time.Now().AddDate(0, 0, 2) // years, months, days
                cookie := http.Cookie{Name: SessionName, Value: sess, Expires: cookieexpire}
                http.SetCookie(w, &cookie)
            }

            fmt.Fprintf(w, email)
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

    http.HandleFunc("/api/GetConnectedServers", func (w http.ResponseWriter, r *http.Request) {
        var activeservers []ActiveServer
        for _, s := range servers {
            if s.connected {
                srv := ActiveServer{UUID: s.uuid, Name: s.name, Playercount: len(s.players)}
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
    http.ListenAndServe(port, nil)
}
