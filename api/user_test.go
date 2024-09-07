package api

import (
	"testing"

	pb "github.com/packetflinger/q2admind/proto"
)

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
