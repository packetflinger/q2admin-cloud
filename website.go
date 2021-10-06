package main

import (
    "fmt"
    "log"
    "net/http"
)

func RunHTTPServer() {
    port := ":27999"
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

    log.Printf("Listening for web requests on %s\n", port)
    http.ListenAndServe(port, nil)
}
