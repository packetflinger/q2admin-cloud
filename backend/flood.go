package backend

import (
	"fmt"
	"time"

	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
)

func TextFlood(player *frontend.Player, message string) {
	now := time.Now().UnixMilli()
	fmt.Println(now)
}

func CheckTextFlood(info *pb.FloodInfo, when int64, tolerance int64) bool {
	info.PrintTotal++
	if when-info.GetLastPrintTime() < tolerance {
		return true
	}
	info.LastPrintTime = when
	return false
}
