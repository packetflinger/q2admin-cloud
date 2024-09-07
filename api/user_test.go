package api

import (
	"testing"

	pb "github.com/packetflinger/q2admind/proto"
)

func TestUserWrite(t *testing.T) {
	users := []User{
		{
			ID:          "81dbcad6-9151-460b-b475-6f728cc2d44d",
			Name:        "claire",
			Email:       "somebody@somewhere.com",
			Description: "Just some weird guy",
		},
		{
			ID:          "55bfac58-5a61-49e1-9ea4-d5c1ab4bfa14",
			Name:        "shloo",
			Email:       "something@somewhere.com",
			Description: "something",
		},
	}

	WriteUsersToDisk(users, "users-test.json")
}

func TestUserRead(t *testing.T) {
	users, err := ReadUsersFromDisk("users-test.json")
	if err != nil {
		t.Error(err)
	}

	if len(users) == 0 {
		t.Log(users)
		t.Error("No users found")
	}
}

/*
func TestUserGet(t *testing.T) {
	users, err := ReadUsersFromDisk("users-test.json")
	if err != nil {
		t.Error(err)
	}
	Q2A.Users = users
	if len(users) == 0 {
		t.Error("No users found")
	}

	u, e := Q2A.GetUser("81dbcad6-9151-460b-b475-6f728cc2d44d")
	if e != nil {
		t.Error(e)
	}
	if u.Name != "claire" {
		t.Error("Name doesn't match")
	}

	u, e = Q2A.GetUserByEmail("somebody@somewhere.com")
	if e != nil {
		t.Error(e)
	}
	if u.Name != "claire" {
		t.Error("Name doesn't match")
	}

	u, e = Q2A.GetUserByName("claire")
	if e != nil {
		t.Error(e)
	}
	if u.Email != "somebody@somewhere.com" {
		t.Error("Name doesn't match")
	}
}
*/

func TestWriteUser(t *testing.T) {
	tests := []struct {
		name    string
		users   []*pb.User
		outfile string
	}{
		{
			name: "test1",
			users: []*pb.User{
				{
					Name:  "user1",
					Email: "user1@example.net",
				},
				{
					Name:  "user2",
					Email: "user2@example.net",
				},
				{
					Name:  "user3",
					Email: "user3@example.net",
				},
			},
			outfile: "../testdata/write-user-out.pb",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := WriteUsers(tc.users, tc.outfile)
			if err != nil {
				t.Error(err)
			}
		})
	}
}
