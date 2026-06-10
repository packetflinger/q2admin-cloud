package backend

import "testing"

func TestFont(t *testing.T) {
	tests := []struct {
		name string
		attr []int
		text string
		want string
	}{
		{
			name: "just foreground color",
			attr: []int{ColorGreen},
			text: "hello",
			want: "\033[32mhello\033[m",
		},
		{
			name: "blue foreground, cyan background",
			attr: []int{ColorBlue, bgcolor(ColorCyan)},
			text: "hello",
			want: "\033[34;46mhello\033[m",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Font(tc.attr, tc.text)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
