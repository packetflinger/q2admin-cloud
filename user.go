package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
)

// A user is someone who admins clients via the website.
// This is the in-memory format
type User struct {
	ID          string       // a UUID, immutable
	Email       string       // can change
	Name        string       // main q2 alias, can change
	Description string       // ?
	Avatar      string       // from auth provider
	Disabled    bool         // user globally cut off
	Permissions []UserAccess // client access
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

// A permission structure mapping users to their quake 2 servers.
// Granting another user admin access to a client they're
// not the owner of can be accomplished using these.
type UserAccess struct {
	User       string `json:"User"`       // uuid
	Client     string `json:"Client"`     // uuid
	Permission int    `json:"Permission"` // bitmask
}

// A website session
type UserSession struct {
	ID      string // uuid
	Created int64  // unix timestamp
	Expires int64  // unix timestamp
}

// Get a pointer to a user based on their ID
func (q2a RemoteAdminServer) GetUser(id string) (*User, error) {
	log.Println(q2a.Users)
	for _, u := range q2a.Users {
		//log.Println(u.ID)
		if u.ID == id {
			return &u, nil
		}
	}
	return &User{}, errors.New("user not found")
}

// Get a pointer to a user based on their email
func (q2a RemoteAdminServer) GetUserByEmail(email string) (*User, error) {
	for i := range q2a.Users {
		if q2a.Users[i].Email == email {
			return &q2a.Users[i], nil
		}
	}
	return &User{}, errors.New("user not found")
}

// Get a pointer to a user based on their name
func (q2a RemoteAdminServer) GetUserByName(n string) (*User, error) {
	for i := range q2a.Users {
		if q2a.Users[i].Name == n {
			return &q2a.Users[i], nil
		}
	}
	return &User{}, errors.New("user not found")
}

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
func ReadUsersFromDisk(filename string) ([]User, error) {
	filedata, err := os.ReadFile(filename)
	if err != nil {
		//log.Println("Problems with", name, "skipping")
		return []User{}, errors.New("unable to read file")
	}

	df := []UserDiskFormat{}
	err = json.Unmarshal([]byte(filedata), &df)
	if err != nil {
		log.Println(err)
		return []User{}, errors.New("unable to parse data")
	}

	users := []User{}
	for _, u := range df {
		newuser := User{
			ID:          u.ID,
			Email:       u.Email,
			Name:        u.Name,
			Description: u.Description,
			Disabled:    u.Disabled,
		}
		users = append(users, newuser)
	}

	return users, nil
}

// Write permissions to disk
func WriteAccessToDisk(access []UserAccess, filename string) error {
	filecontents, err := json.MarshalIndent(access, "", "  ")
	if err != nil {
		log.Println(err)
		return err
	}

	err = os.WriteFile(filename, filecontents, 0644)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

// Parse json on disk into a structure
func ReadAccessFromDisk(filename string) ([]UserAccess, error) {
	filedata, err := os.ReadFile(filename)
	if err != nil {
		return []UserAccess{}, errors.New("unable to read file")
	}

	ua := []UserAccess{}
	err = json.Unmarshal([]byte(filedata), &ua)
	if err != nil {
		return []UserAccess{}, errors.New("unable to parse data")
	}

	return ua, nil
}
