// The q2actl program is a command-line binary for managing cloud-admin
package main

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/packetflinger/q2admind/crypto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/packetflinger/q2admind/proto"
)

var (
	host = flag.String("host", "127.0.0.1", "Server IP/Hostname")
	port = flag.Int("port", 9989, "Server port")
	key  = flag.String("key", "id_ecdsa", "Your private key file")
	user = flag.String("user", "", "email address with access")
)

type tokenAuth struct {
	token string
}

func (t tokenAuth) GetRequestMetadata(ctx context.Context, in ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + t.token,
	}, nil
}

func (tokenAuth) RequireTransportSecurity() bool {
	return false // CHANGE THIS TO TRUE LATER
}

// Bearer token: base64("email:unixtimestamp:signature")
func makeToken(email string, privkey *rsa.PrivateKey) string {
	var out []byte
	t := fmt.Appendf(nil, "%s:%d", email, time.Now().Unix())
	sig := crypto.Sign(privkey, t)
	out = append(out, byte(len(t)))
	out = append(out, t...)
	out = append(out, sig...)
	return base64.StdEncoding.EncodeToString(out)
}

func main() {
	flag.Parse()
	privkey, err := crypto.LoadPrivateKey(*key)
	if err != nil {
		log.Fatalf("error loading private key: %v\n", err)
	}
	ta := tokenAuth{
		token: makeToken(*user, privkey),
	}
	log.Println("token:", ta.token)

	addr := fmt.Sprintf("%s:%d", *host, *port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithPerRPCCredentials(ta))
	// conn, err := grpc.NewClient(addr, grpc.WithPerRPCCredentials(ta))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewQ2AdminClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.FetchStatus(ctx, &pb.StatusRequest{Server: "test"})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("UUID: %s", r.GetUuid())
}
