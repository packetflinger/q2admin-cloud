package backend

import (
	"testing"

	"github.com/golang-jwt/jwt"
	pb "github.com/packetflinger/q2admind/proto"
)

/*
func TestCreateSessionToken(t *testing.T) {
	tests := []struct {
		name    string
		user    *pb.User
		secret  []byte
		want    string
		wantErr bool
	}{
		{
			name:    "nil user",
			user:    nil,
			secret:  []byte("han shot first"),
			want:    "",
			wantErr: true,
		},
		{
			name:    "nil secret",
			user:    &pb.User{Email: "claire@q2.wtf"},
			secret:  nil,
			want:    "",
			wantErr: true,
		},
		{
			name:    "valid",
			user:    &pb.User{Email: "claire@q2.wtf"},
			secret:  []byte("96b30e93fa4ea5f6a915e48da1da0d3c"),
			want:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjg2NDAxNzYwNDA1OTE4LCJqdGkiOiJ0aGlzIGlzIGFuIGlkIiwic3ViIjoiY2xhaXJlQHEyLnd0ZiJ9.d8xkoUf6RVhQ35YrVRTbUdyUc2kKJPhQC9OTZARnlyg",
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CreateSessionToken(tc.user, tc.secret)
			if tc.wantErr != (err != nil) {
				t.Errorf("CreateSessionToken(%v, %v) resulted in an error: %v", tc.user, tc.secret, err)
			}
			if got != tc.want {
				t.Errorf("CreateSessionToken(%v, %v) = %q, want %q", tc.user, tc.secret, got, tc.want)
			}
		})
	}
}
*/

func TestValidateSessionToken(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		user       *pb.User
		secret     []byte
		wantValid  bool
		wantClaims *jwt.StandardClaims
		wantErr    bool
	}{
		{},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

		})
	}
}
