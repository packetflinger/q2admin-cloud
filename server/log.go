package server

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/packetflinger/q2admind/client"
	pb "github.com/packetflinger/q2admind/proto"
	"github.com/packetflinger/q2admind/util"
	"google.golang.org/protobuf/encoding/prototext"
)

// A player said something, record to use against them later
func LogChat(cl *client.Client, chat string) {
	s := "INSERT INTO chat (uuid, chat_time, chat) VALUES (?,?,?)"
	_, err := DB.Exec(s, cl.UUID, util.GetUnixTimestamp(), chat)
	if err != nil {
		log.Println(err)
	}
}

// Save frags for stats
func LogFrag(cl *client.Client, victim int, attacker int) {
	/*
	   sql := "INSERT INTO frag (victim,attacker,server,fragdate) VALUES (?,?,?,?)"
	   _, err := DB.Exec(sql, server, logtype, logentry, now)

	   	if err != nil {
	   		log.Println(err)
	   		return
	   	}
	*/
}

// Insert client-specific event
func LogEvent(cl *client.Client, event string) {
	s := "INSERT INTO client_log (uuid, event_time, event) VALUES (?,?,?)"
	_, err := DB.Exec(s, cl.UUID, util.GetUnixTimestamp(), event)
	if err != nil {
		log.Println(err)
	}
}

// A player connected, save them in the database
//
// Called from ParseConnect()
func LogPlayer(cl *client.Client, pl *client.Player) {
	s := "INSERT INTO player (server, name, ip, hash, userinfo, connect_time) VALUES (?,?,?,?,?,?)"
	_, err := DB.Exec(
		s,
		cl.UUID,
		pl.Name,
		pl.IP,
		pl.UserInfoHash,
		pl.Userinfo,
		util.GetUnixTimestamp(),
	)
	if err != nil {
		log.Println(err)
	}
}

// Insert an system event into the db
func LogSystemEvent(event string) {
	s := "INSERT INTO system_log (log_time, log_entry) VALUES (?,?)"
	_, err := DB.Exec(s, util.GetUnixTimestamp(), event)
	if err != nil {
		log.Println(err)
	}
}

// Each client has it's own logger and dedicated log file along side
// the client's other files. This log is generally just lines of raw
// text.
//
// Open the file and return a logger object for it.
func NewClientLogger(cl *client.Client) (*log.Logger, error) {
	logfile := path.Join(Cloud.Config.ClientDirectory, cl.Name, "log")
	fp, err := os.OpenFile(logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	cl.LogFile = fp
	flags := log.Ldate | log.Ltime | log.Lmicroseconds
	return log.New(fp, "", flags), nil
}

// Get a new logger for the cloud admin server itself. Don't bother keeping
// track of the file pointer, this log will be open as long as the program
// is running.
func NewServerLogger(filename string) (*log.Logger, error) {
	fp, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	return log.New(fp, "", 0), nil
}

// Build a log proto and write it to the logger
func logServerEvent(cl *client.Client, sev pb.LogSeverity, lc pb.LogContext, format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	client := ""
	if cl != nil {
		client = cl.UUID
	}
	entry := &pb.LogEntry{
		Time:     util.GetUnixTimestamp(),
		Client:   client,
		Severity: sev,
		Context:  lc,
		Entry:    msg,
	}
	textpb, err := prototext.Marshal(entry)
	if err != nil {
		fmt.Println("error marshalling log:", err)
		return
	}
	if Cloud.Logger != nil {
		Cloud.Logger.Println(string(textpb))
	}
}
