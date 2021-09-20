package main

import (
    "fmt"
    "net/http"
)

func RunHTTPServer() {
    //fs := http.FileServer(http.Dir("static/"))
    //http.Handle("/static/", http.StripPrefix("/static/", fs))
    http.HandleFunc("/", func (w http.ResponseWriter, r *http.Request) {
        for _, s := range servers {
            if s.connected {
                fmt.Fprintln(w, s)
            }
        }
        fmt.Fprintf(w, "")
    })

    http.ListenAndServe(":27999", nil)
}
