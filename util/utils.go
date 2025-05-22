package util

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	uuid "github.com/google/uuid"
)

func GenerateUUID() string {
	return uuid.NewString()
}

// Dates are stored in the database as unix timestamps
func GetUnixTimestamp() int64 {
	return time.Now().Unix()
}

// Get current time in HH:MM:SS format
func GetTimeNow() string {
	now := time.Now()
	return fmt.Sprintf("%02d:%02d:%02d", now.Hour(), now.Minute(), now.Second())
}

// Convert unix timestamp to a time struct
func GetTimeFromTimestamp(ts int64) time.Time {
	return time.Unix(ts, 0)
}

// TimeAgo gives you a string of how long ago something was
// based on a unix timestamp.
// Examples:
//
//	just now
//	30s ago
//	5m ago
//	2h ago
//	3d ago
//	8w ago
//	2mo ago
//	3yr ago
func TimeAgo(ts int64) string {
	elapsed := GetUnixTimestamp() - ts
	if elapsed < 0 {
		return "soon"
	}
	if elapsed < 5 {
		return "just now"
	}
	if elapsed < 60 {
		return fmt.Sprintf("%ds", elapsed)
	}
	if elapsed < 3600 {
		return fmt.Sprintf("%dm", elapsed/60)
	}
	if elapsed < 86400 {
		return fmt.Sprintf("%dh", elapsed/3600)
	}
	if elapsed < 86400*7 {
		return fmt.Sprintf("%dd", elapsed/86400)
	}
	if elapsed < 86400*30 {
		return fmt.Sprintf("%dw", elapsed/(86400*7))
	}
	if elapsed < 86400*30*52 {
		return fmt.Sprintf("%dy", elapsed/(86400*30))
	}
	return "forever ago"
}

// SortUserinfoKeys will return a list of all the keys in the userinfo map
// associated with a player in alphabetical order.
func SortUserinfoKeys(uiMap map[string]string) []string {
	var out []string
	for k := range uiMap {
		out = append(out, k)
	}
	slices.SortFunc(out, func(a, b string) int {
		return cmp.Compare(a, b)
	})
	return out
}
