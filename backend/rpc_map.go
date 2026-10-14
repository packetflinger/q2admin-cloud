package backend

import (
	"context"
	"fmt"

	pb "github.com/packetflinger/q2admind/proto"
)

func (s *RPCServer) FetchMaplist(ctx context.Context, req *pb.FetchMaplistRequest) (*pb.FetchMaplistResponse, error) {
	valid, ident, err := checkRPCAuthorization(ctx)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, fmt.Errorf("unauthorized")
	}
	be.Logf(LogLevelInfo, "FetchMaplist called for %q", ident)
	name := req.GetServer()
	if name == "" {
		return nil, fmt.Errorf("blank server name in request")
	}
	fe, err := be.FindFrontendByName(name)
	if err != nil {
		return nil, fmt.Errorf("frontend not found")
	}
	return &pb.FetchMaplistResponse{
		Server:   fe.Name,
		Rotation: fe.Maplist.ToProto(),
	}, nil
}
