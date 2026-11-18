package backend

import (
	"strings"
	"testing"

	"github.com/packetflinger/q2admind/frontend"
)

func TestDashboardEmptyState(t *testing.T) {
	data := samplePage()
	data.Frontends = nil
	out := renderPage(t, "dashboard.tmpl", "dashboard", data)
	if !strings.Contains(out, "Add your first server") {
		t.Error("empty dashboard should prompt to add a server")
	}
}

func TestDashboardVersions(t *testing.T) {
	data := samplePage()
	data.Frontends[0].ServerVars["version"] = "r1q2ded b8012"
	data.Frontends[0].ServerVars["q2admin"] = "r1306~c96d8e1"
	data.Frontends[0].ServerVars["gamename"] = "openffa"
	out := renderPage(t, "dashboard.tmpl", "dashboard", data)
	for _, want := range []string{">Versions<", "r1q2ded b8012", "r1306~c96d8e1", "openffa"} {
		if !strings.Contains(out, want) {
			t.Errorf("dashboard is missing %q", want)
		}
	}

	// a numeric revision replaces the build date
	data.Frontends[0].ServerVars["gamename"] = "opentdm"
	data.Frontends[0].ServerVars["gamedate"] = "Nov 20 2023"
	data.Frontends[0].ServerVars["revision"] = "158"
	out = renderPage(t, "dashboard.tmpl", "dashboard", data)
	if !strings.Contains(out, "(r158)") || strings.Contains(out, "Nov 20 2023") {
		t.Error("dashboard should show OpenTDM's revision instead of the date")
	}

	// offline servers are left out, and with none online so is the table
	offline := &frontend.Frontend{Name: "down", UUID: "dead", ServerVars: map[string]string{"version": "q2proded old"}}
	data.Frontends = append(data.Frontends, offline)
	out = renderPage(t, "dashboard.tmpl", "dashboard", data)
	if strings.Contains(out, "q2proded old") {
		t.Error("versions table lists an offline server")
	}
	data.Frontends = []*frontend.Frontend{offline}
	out = renderPage(t, "dashboard.tmpl", "dashboard", data)
	if strings.Contains(out, ">Versions<") {
		t.Error("versions table shown with no servers online")
	}
}
