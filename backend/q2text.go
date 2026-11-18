// Quake 2 text for display. The game's font has 256 characters: the top half
// (128-255) repeats the bottom half in a highlight color, and a few low ones
// are symbols. Mods use the highlighted half for headers and emphasis, like
// OpenTDM's end-of-match stats. Those bytes arrive here as the Latin-1
// characters U+0080 to U+00FF (libq2's ReadString decodes each byte as one),
// so "Item" in highlight shows up as "Éôåí".
//
// These convert game text the way q2pro's dedicated server console does
// (Q_charascii): the highlight bit is dropped so the text reads normally, and
// symbols become ASCII lookalikes. Use them where game text is shown, not
// where it's stored, so logs, the database and rules keep the original.
package backend

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// HighlightColor is how the console shows text the game draws highlighted.
// Quake 2's highlighted characters are green.
const HighlightColor = ColorGreen

// q2Char converts one character of game text: what to show for it, and
// whether the game draws it highlighted.
func q2Char(r rune) (rune, bool) {
	if r > 0xff { // not from the game's font
		return r, false
	}
	switch r {
	case ' ', '\f', '\n', '\r', '\t', '\v':
		return r, false
	}
	highlighted := r >= 0x80
	c := r & 0x7f
	switch {
	case c >= ' ' && c < 0x7f:
		return c, highlighted
	case c == 16:
		return '[', highlighted
	case c == 17:
		return ']', highlighted
	case c >= 29 && c <= 31: // the pieces of a horizontal bar
		return '-', highlighted
	}
	return '.', highlighted // control characters, including escapes
}

// q2Plain is game text made readable, without color: for padded columns and
// anywhere color codes don't belong.
func q2Plain(s string) string {
	return strings.Map(func(r rune) rune {
		c, _ := q2Char(r)
		return c
	}, s)
}

// q2Text is game text made readable for a console, with the characters the
// game highlights in HighlightColor. fg is the color the text sits in, which
// comes back after each highlighted run (ColorDefault for none).
func q2Text(s string, fg int) string {
	if fg == ColorDefault {
		fg = 39 // ANSI's default foreground
	}
	var b strings.Builder
	in := false
	for _, r := range s {
		c, hl := q2Char(r)
		if hl != in {
			if hl {
				b.WriteString("\033[" + strconv.Itoa(HighlightColor) + "m")
			} else {
				b.WriteString("\033[" + strconv.Itoa(fg) + "m")
			}
			in = hl
		}
		b.WriteRune(c)
	}
	if in {
		b.WriteString("\033[" + strconv.Itoa(fg) + "m")
	}
	return b.String()
}

// visibleWidth is how many characters of s show in a terminal, skipping ANSI
// escape sequences: colors (ESC [ ... letter) and links (ESC ] ... ESC \).
func visibleWidth(s string) int {
	n := 0
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) {
			switch s[i+1] {
			case '[':
				j := i + 2
				for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
					j++
				}
				i = j + 1
				continue
			case ']':
				end := strings.Index(s[i+2:], "\x1b\\")
				if end < 0 {
					return n
				}
				i += 2 + end + 2
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
		n++
	}
	return n
}
