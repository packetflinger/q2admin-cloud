package backend

import (
	"testing"

	"github.com/packetflinger/q2admind/frontend"
)

func TestFindFrontend(t *testing.T) {
	tests := []struct {
		name      string
		lookup    string
		frontends []frontend.Frontend
		want      frontend.Frontend
	}{
		{
			name:   "test1",
			lookup: "4bbd2bde-9ab3-4134-8024-93a67db3b3d3",
			frontends: []frontend.Frontend{
				{
					Name: "fe1",
					UUID: "83bfa705-7257-4843-b83a-9bc3ad2a3dfc",
				},
				{
					Name: "fe2",
					UUID: "4bbd2bde-9ab3-4134-8024-93a67db3b3d3",
				},
			},
			want: frontend.Frontend{
				Name: "fe2",
				UUID: "4bbd2bde-9ab3-4134-8024-93a67db3b3d3",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (&Backend{frontends: tc.frontends}).FindFrontend(tc.lookup)
			if err != nil {
				t.Error(err)
			}
			if got.Name != tc.want.Name {
				t.Errorf("FindFrontend(%q) = %v want %v\n", tc.lookup, got, tc.want)
			}
		})
	}
}

func TestFindFrontendByName(t *testing.T) {
	tests := []struct {
		name      string
		lookup    string
		frontends []frontend.Frontend
		want      frontend.Frontend
	}{
		{
			name:   "test1",
			lookup: "fe2",
			frontends: []frontend.Frontend{
				{
					Name: "fe1",
					UUID: "83bfa705-7257-4843-b83a-9bc3ad2a3dfc",
				},
				{
					Name: "fe2",
					UUID: "4bbd2bde-9ab3-4134-8024-93a67db3b3d3",
				},
			},
			want: frontend.Frontend{
				Name: "fe2",
				UUID: "4bbd2bde-9ab3-4134-8024-93a67db3b3d3",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (&Backend{frontends: tc.frontends}).FindFrontendByName(tc.lookup)
			if err != nil {
				t.Error(err)
			}
			if got.Name != tc.want.Name {
				t.Errorf("FindFrontendByName(%q) = %v want %v\n", tc.lookup, got, tc.want)
			}
		})
	}
}
