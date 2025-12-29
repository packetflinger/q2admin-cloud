// This is the RPC server for the backend implementing Google's gRPC interface.
// Each RPC accepts a request proto and returns a result proto.
package backend

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/packetflinger/q2admind/crypto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/packetflinger/q2admind/proto"
)

const (
	AuthorizationHeader = "authorization"
)

var (
	// Generic error messages
	ErrMissingMetadata    = errors.New("metadata missing from context")
	ErrMissingHeader      = errors.New("authorization header missing")
	ErrInvalidTokenFormat = errors.New("invalid token format")
	ErrNotAuthorized      = errors.New("invalid authorization")
)

// RPCServer is needed to implement the RPC server interface
type RPCServer struct {
	pb.UnimplementedQ2AdminServer
}

// startRPCServer listens for RPC requests from authorized users.
// Authentication is handled using bearer tokens.
func (s *Backend) startRPCServer() {
	port := fmt.Sprintf("%s:%d", s.config.RpcAddress, s.config.RpcPort)
	listener, err := net.Listen("tcp", port) // v4 + v6
	if err != nil {
		log.Println(err)
		return
	}
	defer listener.Close()
	srv := grpc.NewServer()
	pb.RegisterQ2AdminServer(srv, &RPCServer{})
	s.Logf(LogLevelNormal, "listening for RPC clients on %s\n", port)
	for {
		if err := srv.Serve(listener); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}
}

// The http "headers" for an RPC request are contained in metadata associated
// with the context. The bearer token is linked to the request as an
// "authorization" header. This function gets the header from the metadata and
// checks if the token is valid for the request.
//
// Returns
// - bool whether this request is authorized or not
// - email address of the user requesting
// - any errors found
func checkRPCAuthorization(ctx context.Context) (bool, string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false, "", ErrMissingMetadata
	}
	tokens := md.Get(AuthorizationHeader)
	if len(tokens) != 1 {
		return false, "", ErrMissingHeader
	}
	if !strings.HasPrefix(tokens[0], "Bearer ") {
		return false, "", ErrInvalidTokenFormat
	}
	return validateBearerToken(strings.TrimPrefix(tokens[0], "Bearer "))
}

// validateBearerToken will check the token for validity and return whether
// it satisfies the requirments to allow the RPC request.
//
// The token is a base64 encoded string made up of 3 parts:
//  1. The length of the payload.
//  2. The payload itself, which is the user's email and token TTL delimited
//     by a ":". The time-to-life is a unix timestamp of the maximum age
//     allowed for a token's validity.
//     Example: "claire@packetflinger.com:1767019152"
//  3. A cryptographic signature of the payload. The payload is hashed using
//     the SHA256 algorithm and then encrypted using the user's private key.
//
// The TTL is intended to be only a few seconds into the future, effectively
// making the token single-use. This way if a token is somehow sniffed or
// stolen, unless it's used almost immediately, it's useless. The TTL can't be
// updated without re-encrypting the signature, which can only be done by the
// user possessing the private key. The client should be generating a new token
// for each request. If the system running the RPC server's time is skewed, an
// attacker might be able to reuse a stolen token, but that's on the owner of
// that system to ensure the system time is up-to-date.
//
// Check the signature matches by decrypting with the public key, check the
// expiration value is greater than the current unix timestamp, and check that
// the user is allowed to use the RPC at all from the config.
//
// Returns true/false for token validity, the email address of the user
// represented by the token, and any errors that occurred along the way.
func validateBearerToken(t string) (bool, string, error) {
	data, err := base64.StdEncoding.DecodeString(t)
	if err != nil {
		return false, "", ErrInvalidTokenFormat
	}
	length := data[0]
	payload := data[1 : length+1]
	signature := data[length+1:]

	payloadParts := strings.SplitN(string(payload), ":", 2)
	if len(payloadParts) != 2 {
		return false, "", fmt.Errorf("invalid token payload: %q", string(payload))
	}
	email := payloadParts[0]
	ttl := payloadParts[1]
	u, err := be.GetUserByEmail(email)
	if err != nil {
		return false, "", fmt.Errorf("getting user from payload: %v", err)
	}
	if !u.GetAllowRpc() {
		return false, u.GetEmail(), fmt.Errorf("RPC access not allowed for user %q", u.GetEmail())
	}
	expires, err := strconv.ParseInt(ttl, 10, 64)
	if err != nil {
		return false, u.GetEmail(), fmt.Errorf("unable to parse token expiration %q as unix timestamp", ttl)
	}
	if time.Now().Unix() > expires {
		return false, u.GetEmail(), fmt.Errorf("expired token")
	}

	// The `rpc_public_key` property of the User proto message is repeated.
	// This is because PEM encoding expects newline characters at specific
	// points; you can't just use a PEM-encoded public key as a single-line
	// string. In addition, you can't have a string in a proto that contains
	// any newlines. Usually you just read public keys from files, but in this
	// case we wanted to encode them directly in the `User` proto. So, each
	// line of the public key is a separate `rpc_public_key` property, that
	// must be joined together (by newlines) as a single byte slice in order
	// to properly decode the PEM formatting.
	//
	// Alternatively, we could've just encoded the data in base64 or something
	// for a single-line, but the data is already encoded and it's visually
	// obvious what it is, a huge base64 string would be more error prone.
	pubData := []byte(strings.Join(u.GetRpcPublicKey(), "\n"))
	pubPem, _ := pem.Decode(pubData)
	if pubPem == nil {
		return false, u.GetEmail(), fmt.Errorf("invalid public key data")
	}

	// ASN.1 DER encoded key expected, see
	// https://datatracker.ietf.org/doc/html/rfc5280#section-4.1
	// Should start with "-----BEGIN PUBLIC KEY-----"
	public, _ := x509.ParsePKIXPublicKey(pubPem.Bytes)
	pu := public.(*rsa.PublicKey)
	if crypto.VerifySignature(pu, payload, signature) {
		return true, u.GetEmail(), nil
	}
	return false, u.GetEmail(), nil
}

func (s *RPCServer) FetchStatus(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	valid, ident, err := checkRPCAuthorization(ctx)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, fmt.Errorf("unauthorized")
	}
	be.Logf(LogLevelInfo, "FetchStatus called for %q", ident)
	name := req.GetServer()
	if name == "" {
		return nil, fmt.Errorf("blank server name in request")
	}
	fe, err := be.FindFrontendByName(name)
	if err != nil {
		return nil, fmt.Errorf("frontend not found")
	}
	return &pb.StatusResponse{Uuid: fe.UUID}, nil
}
