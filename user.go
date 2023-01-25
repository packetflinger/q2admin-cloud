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
	ID          string // a UUID
	Email       string // auth checked by Google or Discord
	Name        string // main q2 alias
	Description string // ?
	Disabled    bool
}

// These are users who will be admining the clients
type UserDiskFormat struct {
	ID          string `json:"ID"` // uuid
	Email       string `json:"Email"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	Disabled    bool   `json:"Disabled"`
}

// write all User objects to json format on disk
func WriteUsersToDisk(users []User, filename string) {
	df := []UserDiskFormat{}
	for _, u := range users {
		df = append(df, UserDiskFormat(u))
	}

	filecontents, err := json.MarshalIndent(df, "", "  ")
	if err != nil {
		log.Println(err)
	}

	err = os.WriteFile(filename, filecontents, 0644)
	if err != nil {
		log.Println(err)
	}
}

// Read a json file containing users and parse into a struct
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
		users = append(users, User(u))
	}

	return users, nil
}
