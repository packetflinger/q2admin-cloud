package backend

import (
	"fmt"
	"strings"
)

// Reference: https://en.wikipedia.org/wiki/ANSI_escape_code
const (
	BlinkSlow          = 5  // less than 150/min
	BlinkFast          = 6  // faster than 150/min (not widely supported)
	BlinkNone          = 25 // turn off blinking
	ColorDefault       = -1 // don't change when rendering
	ColorReset         = 0
	ColorInversed      = 7 // swap foreground and background
	ColorNotInversed   = 27
	ColorBlack         = 30
	ColorRed           = 31
	ColorGreen         = 32
	ColorYellow        = 33
	ColorBlue          = 34
	ColorMagenta       = 35
	ColorCyan          = 36
	ColorLightGray     = 37
	ColorDarkGray      = 90
	ColorBrightRed     = 91
	ColorBrightGreen   = 92
	ColorBrightYellow  = 93
	ColorBrightBlue    = 94
	ColorBrightMagenta = 95
	ColorBrightCyan    = 96
	ColorWhite         = 97
	FontItalic         = 3
	FontUnderline      = 4
	FontStrike         = 9 // not widely supported
	FontNoUnderline    = 24
	FontPrimary        = 10 // default?
	FontFramed         = 51
	FontEncircled      = 52
	FontNotFramed      = 54 // same v
	FontNotEncircled   = 54 // same ^
	WeightBold         = 1
	WeightFaint        = 2
	WeightNormal       = 22
	AnsiReset          = "\033[m"
)

// Font applies ANSI codes to the string for displaying in a terminal. This
// includes font color, background color, weight, decoration, etc. This doesn't
// change the typeface of the font, only the presentation.
func Font(attr []int, s string) string {
	var strs []string
	for _, a := range attr {
		strs = append(strs, fmt.Sprintf("%d", a))
	}
	return fmt.Sprintf("\033[%sm%s%s", strings.Join(strs, ";"), s, AnsiReset)
}

// Backgrounds are 10 digits higher than foregrounds
func bgcolor(c int) int {
	return c + 10
}

// Convenience func for use in templates
func red(s string) string {
	return Font([]int{ColorRed}, s)
}

func green(s string) string {
	return Font([]int{ColorGreen}, s)
}

func yellow(s string) string {
	return Font([]int{ColorYellow}, s)
}

func magenta(s string) string {
	return Font([]int{ColorMagenta}, s)
}

func underline(s string) string {
	return Font([]int{FontUnderline}, s)
}
