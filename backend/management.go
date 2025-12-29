// This is the RPC server for the backend
package backend

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/packetflinger/q2admind/proto"
)

var (
	ErrMissingMetadata    = errors.New("metadata missing from context")
	ErrMissingHeader      = errors.New("authorization header missing")
	ErrInvalidTokenFormat = errors.New("invalid token format")
)

type RPCServer struct {
	pb.UnimplementedQ2AdminServer
}

func (s *Backend) startManagement() {
	port := fmt.Sprintf("%s:%d", be.config.ManagementAddress, be.config.ManagementPort)
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
// "authorization" header. This gets the header from the metadata and checks
// if the token is valid for the request.
func checkRPCAuthorization(ctx context.Context) (bool, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		log.Println("missing metadata")
		return false, ErrMissingMetadata
	}
	tokens := md.Get("authorization")
	if len(tokens) != 1 {
		log.Println("no authorization header")
		return false, ErrMissingHeader
	}
	if !strings.HasPrefix(tokens[0], "Bearer ") {
		return false, ErrInvalidTokenFormat
	}

	log.Println("token at server:", tokens[0])
	return validateBearerToken(strings.TrimPrefix(tokens[0], "Bearer "))
}

func validateBearerToken(t string) (bool, error) {
	data, err := base64.StdEncoding.DecodeString(t)
	if err != nil {
		return false, ErrInvalidTokenFormat
	}
	length := data[0]
	payload := data[1 : length+1]
	//signature := data[length+1:]

	log.Println("payload", string(payload))

	// figure out the public key for this email and validate the signature
	return true, nil
}

func (s *RPCServer) FetchStatus(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	_, _ = checkRPCAuthorization(ctx)
	log.Println("FetchStatus RPC called")
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
