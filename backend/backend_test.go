package backend

import (
	"fmt"
	"testing"

	"github.com/packetflinger/q2admind/frontend"
)

func TestFindFrontend(t *testing.T) {
	tests := []struct {
		name      string
		lookup    string
		frontends []*frontend.Frontend
		want      *frontend.Frontend
	}{
		{
			name:   "test1",
			lookup: "4bbd2bde-9ab3-4134-8024-93a67db3b3d3",
			frontends: []*frontend.Frontend{
				{
					Name: "fe1",
					UUID: "83bfa705-7257-4843-b83a-9bc3ad2a3dfc",
				},
				{
					Name: "fe2",
					UUID: "4bbd2bde-9ab3-4134-8024-93a67db3b3d3",
				},
			},
			want: &frontend.Frontend{
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
		frontends []*frontend.Frontend
		want      *frontend.Frontend
	}{
		{
			name:   "test1",
			lookup: "fe2",
			frontends: []*frontend.Frontend{
				{
					Name: "fe1",
					UUID: "83bfa705-7257-4843-b83a-9bc3ad2a3dfc",
				},
				{
					Name: "fe2",
					UUID: "4bbd2bde-9ab3-4134-8024-93a67db3b3d3",
				},
			},
			want: &frontend.Frontend{
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

func TestFrontendPointerSurvivesAppend(t *testing.T) {
	b := &Backend{frontends: []*frontend.Frontend{{Name: "fe1"}}}
	before, err := b.FindFrontendByName("fe1")
	if err != nil {
		t.Fatal(err)
	}
	for range 10 { // force the slice to reallocate
		b.frontends = append(b.frontends, &frontend.Frontend{})
	}
	before.Connected = true
	after, _ := b.FindFrontendByName("fe1")
	if after != before || !after.Connected {
		t.Error("pointer from FindFrontendByName went stale after append")
	}
}

func TestFrontendsSnapshotUnderConcurrentChanges(t *testing.T) {
	b := &Backend{frontends: []*frontend.Frontend{{Name: "fe0"}}}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 100 {
			f := &frontend.Frontend{Name: fmt.Sprint("x", i)}
			b.frontendsMu.Lock()
			b.frontends = append(b.frontends, f)
			b.frontendsMu.Unlock()
		}
	}()
	for range 100 {
		for _, f := range b.Frontends() {
			_ = f.Name
		}
		if _, err := b.FindFrontendByName("fe0"); err != nil {
			t.Fatal(err)
		}
	}
	<-done
	if n := len(b.Frontends()); n != 101 {
		t.Errorf("len(Frontends()) = %d, want 101", n)
	}
}
