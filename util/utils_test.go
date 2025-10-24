package util

import (
	"slices"
	"testing"
)

func TestTimeString(t *testing.T) {
	tests := []struct {
		name      string
		timestamp int64
		want      string
	}{
		{
			name:      "test1",
			timestamp: 1761307433,
			want:      "08:03:53",
		},
		{
			name:      "test2",
			timestamp: 1761228554,
			want:      "10:09:14",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TimeString(tc.timestamp)
			if got != tc.want {
				t.Errorf("TimeString(%d) = %q, want %q", tc.timestamp, got, tc.want)
			}
		})
	}
}

func TestSortUserinfoKeys(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
		want  []string
	}{
		{
			name:  "nil",
			input: nil,
			want:  []string{},
		},
		{
			name: "values",
			input: map[string]string{
				"key3": "val3",
				"key1": "val1",
				"key2": "val2",
			},
			want: []string{"key1", "key2", "key3"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SortUserinfoKeys(tc.input)
			if slices.Compare(got, tc.want) != 0 {
				t.Errorf("SortUserinfoKeys(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
