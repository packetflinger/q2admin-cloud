package util

import (
	"cmp"
	"fmt"
	"slices"
	"time"
)

// Obtain an HH:MM:SS formated string for a given unix timestamp
func TimeString(ts int64) string {
	when := time.Unix(ts, 0)
	return fmt.Sprintf("%02d:%02d:%02d", when.Hour(), when.Minute(), when.Second())
}

// TimeAgo gives you a string of how long ago something was based on a unix
// timestamp. The longer ago the timestamp the less accurate we get.
// Examples:
//
//	never
//	just now
//	30s ago
//	2h ago
//	3d ago
//	2mo ago
func TimeAgo(ts int64) string {
	elapsed := time.Now().Unix() - ts
	if elapsed < 0 {
		return "never"
	}
	if elapsed < 5 {
		return "just now"
	}
	if elapsed < 60 {
		return fmt.Sprintf("%ds ago", elapsed)
	}
	if elapsed < 3600 {
		return fmt.Sprintf("%dm ago", elapsed/60)
	}
	if elapsed < 86400 {
		return fmt.Sprintf("%dh ago", elapsed/3600)
	}
	if elapsed < 86400*7 {
		return fmt.Sprintf("%dd ago", elapsed/86400)
	}
	if elapsed < 86400*30 {
		return fmt.Sprintf("%dw ago", elapsed/(86400*7))
	}
	if elapsed < 86400*30*52 {
		return fmt.Sprintf("%dy ago", elapsed/(86400*30))
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
