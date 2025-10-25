package maprotator

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/google/uuid"

	pb "github.com/packetflinger/q2admind/proto"
)

func NewMapRotation(name string, maps []string) *pb.MapRotation {
	if len(maps) == 0 {
		return nil
	}
	if name == "" {
		name = "default rotation"
	}
	rot := &pb.MapRotation{
		Name: name,
		Uuid: uuid.NewString(),
	}
	for _, m := range maps {
		mapEntry := &pb.Map{}
		data := strings.Fields(m)
		if len(data) >= 1 {
			mapEntry.Name = data[0]
		}
		if len(data) >= 2 {
			min, err := strconv.Atoi(data[1])
			if err == nil {
				mapEntry.MinimumPlayers = int32(min)
			}
		}
		if len(data) >= 3 {
			max, err := strconv.Atoi(data[2])
			if err == nil {
				mapEntry.MaximumPlayers = int32(max)
			}
		}
		if len(data) == 4 {
			flags, err := strconv.Atoi(data[3])
			if err == nil {
				mapEntry.Flags = int64(flags)
			}
		}
		rot.Maps = append(rot.Maps, mapEntry)
		rot.Size = int32(len(rot.Maps))
	}
	return rot
}

// Get the next map in the rotation. If we're at the end, start over at the
// beginning.
func Next(rot *pb.MapRotation) *pb.Map {
	if rot.GetSize() == 0 {
		return nil
	}
	fmt.Println("map len", len(rot.GetMaps()))
	fmt.Println("size", rot.GetSize())
	nextIndex := rot.GetIndex() + 1
	if nextIndex == rot.GetSize() {
		nextIndex = 0
	}
	return rot.GetMaps()[nextIndex]
}

// Fisher-Yates shuffle
func Shuffle(rot *pb.MapRotation) *pb.MapRotation {
	maps := rot.GetMaps()
	rand.Shuffle(len(maps), func(i, j int) {
		maps[i], maps[j] = maps[j], maps[i]
	})
	rot.Maps = maps
	return rot
}
