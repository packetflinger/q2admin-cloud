package maprotator

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	pb "github.com/packetflinger/q2admind/proto"
)

func TestMapRotation(t *testing.T) {
	tests := []struct {
		name    string
		maps    []string
		rotname string
		want    *pb.MapRotation
	}{
		{
			name:    "nil",
			maps:    nil,
			rotname: "",
			want:    nil,
		},
		{
			name: "just map names",
			maps: []string{
				"q2dm1",
				"q2dm2",
				"q2dm3",
			},
			want: &pb.MapRotation{
				Name: "default rotation",
				Size: 3,
				Maps: []*pb.Map{
					{Name: "q2dm1"},
					{Name: "q2dm2"},
					{Name: "q2dm3"},
				},
			},
		},
		{
			name: "maps with some metadata",
			maps: []string{
				"q2dm1 0 20",
				"q2dm2 5 40",
				"q2dm3 0 35",
			},
			want: &pb.MapRotation{
				Name: "default rotation",
				Size: 3,
				Maps: []*pb.Map{
					{Name: "q2dm1", MaximumPlayers: 20},
					{Name: "q2dm2", MinimumPlayers: 5, MaximumPlayers: 40},
					{Name: "q2dm3", MaximumPlayers: 35},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NewMapRotation("", tc.maps)
			options := []cmp.Option{
				protocmp.Transform(),
				protocmp.IgnoreFields(&pb.MapRotation{}, "uuid"),
			}
			if diff := cmp.Diff(got, tc.want, options...); diff != "" {
				t.Errorf("NewMapRotation(%v) resulted in proto diff (-want +got):\n%s", tc.maps, diff)
			}
		})
	}
}

func TestNext(t *testing.T) {
	tests := []struct {
		name string
		rot  *pb.MapRotation
		want *pb.Map
	}{
		{
			name: "nil",
			rot:  nil,
			want: nil,
		},
		{
			name: "dm1 to dm2",
			rot: &pb.MapRotation{
				Name:  "name",
				Size:  3,
				Index: 0,
				Maps: []*pb.Map{
					{Name: "q2dm1"},
					{Name: "q2dm2"},
					{Name: "q2dm3"},
				},
			},
			want: &pb.Map{
				Name: "q2dm2",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Next(tc.rot)
			options := []cmp.Option{
				protocmp.Transform(),
				protocmp.IgnoreFields(&pb.MapRotation{}, "uuid"),
			}
			if diff := cmp.Diff(got, tc.want, options...); diff != "" {
				t.Errorf("Next(%v) resulted in proto diff (-want +got):\n%s", tc.rot, diff)
			}
		})
	}
}
