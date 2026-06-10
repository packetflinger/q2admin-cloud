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

type ansiCode struct {
	foreground int
	background int
	bold       bool
	underlined bool
	inversed   bool
}

// Render will build an ANSI color code based on the receiver. This is only
// used when sending strings to an SSH terminal.
func (c ansiCode) Render() string {
	b := WeightNormal
	if c.bold {
		b = WeightBold
	}
	u := FontNoUnderline
	if c.underlined {
		u = FontUnderline
	}
	r := 27
	if c.inversed {
		r = 7
	}
	return fmt.Sprintf("\033[0;%d;%d;%d;%d;%dm", c.foreground, c.background+10, b, u, r)
}

func PrettyString(s string, fg, bg int, b, u bool) string {
	ac := ansiCode{
		foreground: fg,
		background: bg,
		bold:       b,
		underlined: u,
	}
	return fmt.Sprintf("%s%s%s", ac.Render(), s, AnsiReset)
}

func Font(attr []int, s string) string {
	var strs []string
	for _, a := range attr {
		strs = append(strs, fmt.Sprintf("%d", a))
	}
	code := strings.Join(strs, ";")
	return fmt.Sprintf("\033[%sm%s%s", code, s, AnsiReset)
}

// Backgrounds are 10 digits higher than foregrounds
func bgcolor(c int) int {
	return c + 10
}

// Convenience func for use in templates
func red(s string) string {
	return PrettyString(s, ColorRed, 0, false, false)
}

func green(s string) string {
	return PrettyString(s, ColorGreen, 0, false, false)
}

func yellow(s string) string {
	return PrettyString(s, ColorYellow, 0, false, false)
}

func magenta(s string) string {
	return PrettyString(s, ColorMagenta, 0, false, false)
}

func underline(s string) string {
	return Font([]int{FontUnderline}, s)
}
