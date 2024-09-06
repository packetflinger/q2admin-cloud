package api

import (
	"encoding/json"
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

// These are users who will be admining the clients
type UserDiskFormat struct {
	ID          string `json:"ID"` // uuid
	Email       string `json:"Email"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	Disabled    bool   `json:"Disabled"`
}

// A website session
type UserSession struct {
	ID      string // uuid
	Created int64  // unix timestamp
	Expires int64  // unix timestamp
}

/*
// Get a pointer to a user based on their ID
func (q2a *CloudAdminServer) GetUser(id string) (*pb.User, error) {
	log.Println(q2a.Users)
	for _, u := range q2a.Users {
		//log.Println(u.ID)
		if u.GetUuid() == id {
			return u, nil
		}
	}
	return &pb.User{}, errors.New("user not found")
}
*/

// write all User objects to json format on disk
func WriteUsersToDisk(users []User, filename string) {
	dusers := []UserDiskFormat{}
	for _, u := range users {
		df := UserDiskFormat{}
		df.ID = u.ID
		df.Name = u.Name
		df.Email = u.Email
		df.Description = u.Description
		df.Disabled = u.Disabled
		dusers = append(dusers, df)
	}

	filecontents, err := json.MarshalIndent(dusers, "", "  ")
	if err != nil {
		log.Println(err)
	}

	err = os.WriteFile(filename, filecontents, 0644)
	if err != nil {
		log.Println(err)
	}
}

// Read a json file containing users and parse into a struct.
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
