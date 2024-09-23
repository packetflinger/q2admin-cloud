package server

import "testing"

func TestRender(t *testing.T) {
	tests := []struct {
		name string
		code ansiCode
		want string
	}{
		{
			name: "test 1",
			code: ansiCode{
				foreground: ColorRed,
				background: ColorWhite,
			},
			want: "\033[0;31;107;22;24;27m",
		},
		{
			name: "test 2",
			code: ansiCode{
				//foreground: ColorWhite,
				background: ColorBlue,
				bold:       true,
			},
			want: "\033[0;0;44;1;24;27m",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.code.Render()
			if got != tc.want {
				t.Error("got:", got, "want:", tc.want)
			}
		})
	}
}
