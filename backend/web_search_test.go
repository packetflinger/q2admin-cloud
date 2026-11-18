package backend

import (
	"strings"
	"testing"
)

func TestSearchPages(t *testing.T) {
	data := samplePage()
	out := renderPage(t, "search.tmpl", "search", data)
	if !strings.Contains(out, "/player/YWxwaGFcMTAw") || strings.Contains(out, "<th class=\"d-none d-sm-table-cell\">Server</th>") {
		t.Error("server search should list the result without a Server column")
	}
	data.Frontend = nil
	out = renderPage(t, "search.tmpl", "search", data)
	if !strings.Contains(out, "Search all servers") || !strings.Contains(out, ">pf-tdm</td>") {
		t.Error("global search should show which server each result is from")
	}
	out = renderPage(t, "player.tmpl", "player", data)
	if !strings.Contains(out, "male/grunt") {
		t.Error("player record should show parsed userinfo")
	}
}
