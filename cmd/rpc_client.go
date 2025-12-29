// The rpc_client program is a command-line binary for managing cloud-admin
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
	host     = flag.String("host", "127.0.0.1", "Server IP/Hostname")
	port     = flag.Int("port", 9989, "Server port")
	key      = flag.String("key", "id_ecdsa", "Your private key file")
	user     = flag.String("user", "", "email address with access")
	tokenTTL = flag.Int("token_ttl", 5, "Seconds the token is valid, DON'T CHANGE THIS")
)

type tokenAuth struct {
	token string
}

// GetRequestMetadata generates the metadata (headers) for the request and
// inserts the newly generated bearer token for authorization. This is part of
// the gRPC interface and is called automatically prior to the request being
// sent.
func (t tokenAuth) GetRequestMetadata(ctx context.Context, in ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + t.token,
	}, nil
}

// RequireTransportSecurity tells the gRPC interface whether the connection
// needs to be TLS encrtyped. Using encrypting will prevent attackers from
// being able to sniff the bearer tokens. That risk is minimal however due to
// the token's very short time-to-live.
func (tokenAuth) RequireTransportSecurity() bool {
	return false // CHANGE THIS TO TRUE LATER
}

// makeToken creates a new bearer token for an RPC request. These tokens are
// intended to be one-time-use only but nothing is stopping their re-use other
// than the TTL: base64("email:unixtimestamp:signature")
func makeToken(email string, ttl int64, privkey *rsa.PrivateKey) string {
	var out []byte
	t := fmt.Appendf(nil, "%s:%d", email, time.Now().Unix()+ttl)
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
		token: makeToken(*user, int64(*tokenTTL), privkey),
	}
	addr := fmt.Sprintf("%s:%d", *host, *port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithPerRPCCredentials(ta))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewQ2AdminClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.FetchStatus(ctx, &pb.StatusRequest{Server: "test"})
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	log.Printf("UUID: %s", r.GetUuid())
}
