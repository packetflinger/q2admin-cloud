package api

import (
	"log"
	"os"

	pb "github.com/packetflinger/q2admind/proto"
	"google.golang.org/protobuf/encoding/prototext"
)

// A user is someone who admins clients via the website.
// This is the in-memory format
type User struct {
	ID          string // a UUID, immutable
	Email       string // can change
	Name        string // main q2 alias, can change
	Description string // ?
	Avatar      string // from auth provider
	Disabled    bool   // user globally cut off
	Session     UserSession
}

// A website session
type UserSession struct {
	ID      string // uuid
	Created int64  // unix timestamp
	Expires int64  // unix timestamp
}

// Read a text proto file containing api users and unmarshal it.
//
// Called at startup
func ReadUsersFromDisk(filename string) ([]*pb.User, error) {
	users := []*pb.User{}
	filedata, err := os.ReadFile(filename)
	if err != nil {
		return users, err
	}

	df := &pb.Users{}
	err = prototext.Unmarshal(filedata, df)
	if err != nil {
		return users, err
	}
	return df.GetUser(), nil
}

// Write the user proto to disk
func WriteUsers(users []*pb.User, name string) error {
	userspb := &pb.Users{
		User: users,
	}
	data, err := prototext.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
	}.Marshal(userspb)
	if err != nil {
		return err
	}
	err = os.WriteFile(name, data, 0644)
	if err != nil {
		log.Println(err)
	}
	return nil
}
