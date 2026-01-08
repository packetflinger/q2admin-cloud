package maprotator

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/google/uuid"

	pb "github.com/packetflinger/q2admind/proto"
)

type MapList struct{ *pb.MapRotation }

func NewMapRotation(name string, maps []string) *MapList {
	if len(maps) == 0 {
		fmt.Println("0 maps")
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
	return &MapList{rot}
}

// Get the next map in the rotation. If we're at the end, start over at the
// beginning.
func (m *MapList) Next() *pb.Map {
	if m.GetSize() == 0 {
		return nil
	}
	var nextIndex int32
	switch m.Direction {
	case pb.MapRotationDirection_MapRotationDirectionForward:
		nextIndex = m.GetIndex() + 1
		if nextIndex == m.GetSize() {
			nextIndex = 0
		}
	case pb.MapRotationDirection_MapRotationDirectionReverse:
		nextIndex = m.GetIndex() - 1
		if nextIndex == -1 {
			nextIndex = m.GetSize() - 1
		}
	}
	m.Index = nextIndex
	return m.GetMaps()[nextIndex]
}

// Fisher-Yates shuffle
func (m MapList) Shuffle() MapList {
	maps := m.GetMaps()
	rand.Shuffle(len(maps), func(i, j int) {
		maps[i], maps[j] = maps[j], maps[i]
	})
	m.Maps = maps
	return m
}
