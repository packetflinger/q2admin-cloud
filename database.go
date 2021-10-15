package main

import (
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
    "log"
)

func DatabaseConnect() *sql.DB {
    db, err := sql.Open("mysql", config.DBString)
    if err != nil {
        panic(err)
    }

    if err = db.Ping(); err != nil {
        panic(err)
    }

    return db
}

/**
 * Pull all the servers from the database and load them
 * into a structure
 */
func LoadServers(db *sql.DB) []Server {
    sql := "SELECT id, uuid, owner, name, ip, port, disabled FROM server"
    r, err := db.Query(sql)
    if err != nil {
        panic(err)
    }
    var srvs []Server
    var srv Server
    var disabled int
    for r.Next() {
        r.Scan(&srv.id, &srv.uuid, &srv.owner, &srv.name, &srv.ipaddress, &srv.port, &disabled)
        srv.enabled = disabled == 0
        srvs = append(srvs, srv)
    }
    r.Close()

    return srvs
}

func LogEventToDatabase(server int, logtype int, logentry string) {
    sql := "INSERT INTO logdata (server, msgtype, entry, entrydate) VALUES (?,?,?,NOW())"
    q, err := db.Query(sql, server, logtype, logentry)
    if err != nil {
        log.Println(err)
    }
    q.Close()
}