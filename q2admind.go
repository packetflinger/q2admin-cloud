package main

import (
    //"bufio"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "log"
    "math/rand"
    "net"
    "os"
    "crypto/rsa"
    //"strconv"
    //"strings"
    "time"
)

const versionRequired = 200

// use a custom buffer struct to keep track of where
// we are in the stream of bytes internally
type MessageBuffer struct {
    buffer []byte
    index  int
    length int // maybe not needed
}

type Player struct {
    clientid     int
    name         string
    userinfo     string
    frags        int
    deaths       int
    suicides     int
    teleports    int
    lastteleport int
}

// this is a Quake 2 Gameserver, and also a client to us
type Server struct {
    id         int // this is the database index
    key        int
    index      int
    version    int // what version are we running
    name       string
    ipaddress  string
    port       int // default 27910
    connected  bool
    currentmap string
    enabled    bool
    connection *net.Conn
    players    []Player
    message    MessageBuffer
    messageout MessageBuffer
    encrypted  bool
    publickey  *rsa.PublicKey
    aeskey     []byte          // 16 (128bit)
    nonce      []byte          // 12 for gcm
    challenge  []byte
}

// "this" admin server
type AdminServer struct {
    privatekey  *rsa.PrivateKey
    publickey   *rsa.PublicKey
}

// structure of the config file
type Config struct {
    Address string
    Port int
    Database   string
    PrivateKey string
    APIPort    int
}

var config Config
var q2a AdminServer

const (
    _ = iota
    CMDHello
    CMDQuit
    CMDConnect
    CMDDisconnect
    CMDPlayerList
    CMDPlayerUpdate
    CMDPrint
    CMDCommand
    CMDPlayers
    CMDFrag
    CMDMap
    CMDPing
    CMDAuth
)

const (
    _ = iota
    SCMDHelloAck
    SCMDError
    SCMDPong
    SCMDComand
    SCMDSayClient
    SCMDSayAll
    SCMDAuth
    SCMDTrusted
    SCMDKey
)

var servers = []Server {
    {id: 1, key:1234, name: "dm", ipaddress: "107.174.230.210", port: 27910, enabled: true},
    {id: 2, key:2345, name: "dmx", ipaddress: "107.174.230.210", port: 27911, enabled: true},
    {id: 3, key:4567, name: "tourney", ipaddress: "107.174.230.210", port: 27912, enabled: true},
    {id: 4, key:5678, name: "tourney2", ipaddress: "107.174.230.210", port: 27913, enabled: true},
}

func clearmsg(msg *MessageBuffer) {
    msg.buffer = []byte{0x00}
    msg.index = 0
    msg.length = 0
}

func parseMessage(msg *MessageBuffer) {
    /*
    for {
        if msg.index >= len(msg.buffer) {
            break
        }

        switch b := ReadByte(msg); b {
        case CMDHello:
            //parseHello(msg)
        }
    }
    */
}

/**
 * Locate the struct of the server for a particular
 * ID, get a pointer to it
 */
func findserver(lookup int) (*Server, error) {
    for _, srv := range(servers) {
        if srv.key == lookup {
            return &srv, nil
        }
    }

    return nil, errors.New("Unknown server")
}

/**
 * Setup the connection
 * The first message sent should identify the game server
 * and trigger the authentication process
 */
func handleConnection(c net.Conn) {
    fmt.Printf("Serving %s\n", c.RemoteAddr().String())

    input := make([]byte, 5000)
    var msg MessageBuffer

    bytesread, _ := c.Read(input)
    msg.buffer = input

    fmt.Printf("Read Input:\n%s\n\n", hex.Dump(input[0:bytesread]))

    magic := ReadLong(&msg)
    if magic != 1128346193 {
        // not a valid client, just close connection
        c.Close()
        return
    }
    log.Println("Magic value accepted")

    _ = ReadByte(&msg) // should be CMDHello
    key := ReadLong(&msg)
    ver := ReadLong(&msg)
    port := ReadShort(&msg)
    _ = ReadByte(&msg) // max players
    enc := ReadByte(&msg)
    nonce := ReadData(&msg, 16)

    if ver < versionRequired {
        // write an error, close, return
        c.Close()
        return
    }
    log.Println("Running acceptable version")

    server, err := findserver(int(key))
    if err != nil {
        // write an error, close socket, returns
        log.Println(err)
        c.Close()
        return
    }
    log.Printf("Server located: %s\n", server.name)

    server.port = int(port)
    server.encrypted = int(enc) == 1    // stupid bool conversion
    server.nonce = nonce
    server.connection = &c
    server.connected = true
    keyname := fmt.Sprintf("keys/%d.pem", key)

    log.Printf("Trying to load public key: %s\n", keyname)
    pubkey, err := LoadPublicKey(keyname)
    server.publickey = pubkey
    if err != nil {
        log.Printf("Loading public key: %s\n", err.Error())
    }

    challengeCipher := Sign(q2a.privatekey, server.nonce)
    WriteByte(SCMDHelloAck, &server.messageout)
    WriteShort(len(challengeCipher), &server.messageout)
    WriteData(challengeCipher, &server.messageout)

    /**
     * if client requests encrypted transit, encrypt the session key/iv
     * with the client's public key to keep it confidential
     */
    if server.encrypted {
        aeskey := RandomBytes(16)
        aesiv := RandomBytes(16)
        blob := append(aeskey, aesiv...)
        aescipher := PublicEncrypt(server.publickey, blob)
        WriteData(aescipher, &server.messageout)
    }

    chal := RandomBytes(16)
    server.challenge = chal
    WriteData(server.challenge, &server.messageout)

    c.Write(server.messageout.buffer)

    // read the client signature
    size, _ := c.Read(input)
    msg.buffer = input
    msg.index = 0
    msg.length = size

    op := ReadByte(&msg)    // should be CMDAuth (0x0d)
    if op != CMDAuth {
        c.Close()
        return
    }

    sigsize := ReadShort(&msg)
    clientSignature := ReadData(&msg, int(sigsize))
    verified := VerifySignature(server.publickey, server.challenge, clientSignature)

    if verified {
        fmt.Printf("%s signature verified\n", server.name)
    } else {
        fmt.Printf("%s signature verifcation failed...", server.name)
        c.Close()
        return
    }

    for {
        size, err := c.Read(input)
        if err != nil {
            log.Printf(
                "%s disconnected: %s\n",
                c.RemoteAddr().String(),
                err.Error())
            break;
        }

        msg.buffer = input
        msg.index = 0
        msg.length = size

        fmt.Printf("Read:\n%s\n\n", hex.Dump(input[0:size]))
        //parseMessage(&msgbuf)
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

    privkey, _ := LoadPrivateKey(config.PrivateKey)
    pubkey := privkey.Public().(*rsa.PublicKey)
    q2a.privatekey = privkey
    q2a.publickey = pubkey
}
