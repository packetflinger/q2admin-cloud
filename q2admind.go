package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "log"
    "math/rand"
    "net"
    "os"
    "crypto/rsa"
    "strconv"
    "strings"
    "time"
)

// use a custom buffer struct to keep track of where
// we are in the stream of bytes internally
type MessageBuffer struct {
    buffer []byte
    index  int32
    length int32 // maybe not needed
}

type Player struct {
    clientid     int8
    name         string
    userinfo     string
    frags        int16
    deaths       int16
    suicides     int16
    teleports    int16
    lastteleport int32
}

// this is a Quake 2 Gameserver, and also a client to us
type Server struct {
    id         int32 // this is the database index
    key        int32
    index      int32
    version    int32 // what version are we running
    name       string
    ipaddress  string
    port       int16 // default 27910
    connected  bool
    currentmap string
    enabled    bool
    connection *net.Conn
    players    []Player
    message    MessageBuffer
    encrypted  bool
    publickey  *rsa.PublicKey
    aeskey     []byte          // 16 (128bit)
    nonce      []byte          // 12 for gcm
}

// structure of the config file
type Config struct {
    Address string
    Port int
    Database   string
    Privatekey string
    APIPort    int
}

var config Config

var Servers = []Server {
    {id: 1, key:1234, name: "dm", ipaddress: "107.174.230.210", port: 27910, enabled: true},
    {id: 2, key:2345, name: "dmx", ipaddress: "107.174.230.210", port: 27911, enabled: true},
    {id: 3, key:4567, name: "tourney", ipaddress: "107.174.230.210", port: 27912, enabled: true},
    {id: 4, key:5678, name: "tourney2", ipaddress: "107.174.230.210", port: 27913, enabled: true},
}

func handleConnection(c net.Conn) {
    fmt.Printf("Serving %s\n", c.RemoteAddr().String())
    for {
        netData, err := bufio.NewReader(c).ReadString('\n')
        if err != nil {
            fmt.Println(err)
            return
        }

        temp := strings.TrimSpace(string(netData))
        if temp == "STOP" {
            break
        }

        result := strconv.Itoa(rand.Intn(100)) + "\n"
        c.Write([]byte(string(result)))
    }
    c.Close()
}

func main() {
    port := fmt.Sprintf("%s:%d", config.Address, config.Port)
    listener, err := net.Listen("tcp", port) // v4 + v6
    if err != nil {
        fmt.Println(err)
        return
    }
    defer listener.Close()

    for {
        c, err := listener.Accept()
        if err != nil {
            fmt.Println(err)
            return
        }
        go handleConnection(c)
    }
}

func init() {
    configfile := "q2a.json" // override with cli arg
    if len(os.Args) > 1 {
        configfile = os.Args[1]
    }

    confjson, err := os.ReadFile(configfile)
    if err != nil {
        log.Fatal(err)
    }

    err = json.Unmarshal(confjson, &config)
    if err != nil {
        log.Fatal(err)
    }

    rand.Seed(time.Now().Unix())
    // testing stuffb
    //public, err := LoadPublicKey("public.pem")
    //if err != nil {
    //    fmt.Println(err)
    //}
    //fmt.Println(public)
    //os.Exit(1)
    /*
    for _, srv := range(Servers) {
        fmt.Printf("%d - %s - %s:%d\n", srv.id, srv.name, srv.ipaddress, srv.port)
    }
    */
}
