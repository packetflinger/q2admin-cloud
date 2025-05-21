package server

import "fmt"

const (
	ColorReset         = 0
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
	b := 22
	if c.bold {
		b = 1
	}
	u := 24
	if c.underlined {
		u = 4
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
