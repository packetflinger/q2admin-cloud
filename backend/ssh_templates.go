// Text templates for SSH command output.

package backend

import (
	"strings"
	"text/template"
	"time"

	"github.com/packetflinger/q2admind/database"
	"github.com/packetflinger/q2admind/util"
)

type HelpCommands struct {
	Cmds []struct {
		Cmd  string
		Desc string
	}
	Extra string
}

type SearchResultsOutput struct {
	Query   string
	Results []database.SearchResult
}

const (
	helpTemplate = `
{{ printf "Available commands" | underline }}:
{{- range .Cmds}}
  {{ printf "%-20s" .Cmd }}                   {{ .Desc -}}
{{end}}

{{.Extra}}
`
	statusTemplate = `
{{ printf "Frontend Status" | underline }}:
Frontend:      {{ .IPAddress }}:{{ .Port }}
Peer:          {{ with .Connection }}{{ .RemoteAddr.String | magenta }}{{ else }}not connected{{ end }}
Current map:   {{ .CurrentMap }}
Previous map:  {{ .PreviousMap }}
Invite Tokens: {{ .Invites.Tokens }}/{{ .Invites.Max }}
Invites Used:  {{ .Invites.UseCount }}
Teleports:     {{ .TeleportCount }}

{{ if online . -}}
num score name            vpn address
--- ----- --------------- --- ------------------------------
{{ range .Players }}{{ if .ConnectTime -}}
{{ printf "%3d" .ClientID }} {{ printf "%5d" .Frags }} {{ printf "%-15s" (q2 .Name) }}  {{ .VPN | checkMark | red }}  {{ .IP }}
{{ end }}{{ end -}}
{{ else -}}
No UDP clients
{{ end -}}
`

	searchTemplate = `
Search results for "{{ .Query }}"
name             server              seen  address
---------------  ---------------  -------  -------------------------
{{ range .Results -}}
{{ printf "%-15s" (q2 .Name) }}  {{ printf "%-15s" .Server}}  {{ printf "%7s" .Ago }}  {{ .IP }}
{{ end }}
`
	rulesTemplate = `
  ID        Type     Description
  --------  -------  -----------------------------------------------------
  {{ range . -}}
  {{ if .GetUuid }}{{ slice .GetUuid 0 8}}  {{ printf "%-7s" .GetType }}  {{ join .GetDescription " " | truncate 53 }}{{ end }}
  {{ end }}
`

	whoisTemplate = `
{{ printf "Player Information" | underline}}:
  Name:    "{{ q2 .Name }}"
  IP:       {{ .IP }} 
  DNS:      {{ .Hostname }}
  Version:  {{ .Version }}
  VPN:      {{ .VPN }}

{{ printf "Userinfo Data" | underline}}:
{{ range $k, $v := .UserinfoMap -}}
{{ printf "%15s" $k}} = {{ $v }}
{{ end -}}


{{ printf "Rules Matching" | underline}}:
id        type     description
--------  -------  -----------------------------------------------------
{{ range .Rules -}}
{{ slice .GetUuid 0 8}}  {{ printf "%-7s" .GetType }}  {{ join .GetDescription " " | truncate 53 }}
{{ end }}
`

	serversTemplate = `
{{ printf "Your servers" | underline }}:
Name                  Status     Ver  Time      Peer
--------------------  ---------  ---- --------  ------------------------------------------
{{ range . -}}
{{ printf "%-20s" .Name }}  {{ printf "%-9s" (. | connected) }}  {{ if .Connection }}{{ printf "%4d" .Version }}{{ else }}{{ printf "    " }}{{ end}} {{ if .Connection }}{{ printf "%-8s" (.ConnectTime | ago)}}  {{ .Connection.RemoteAddr.String }}{{ end }}
{{ end -}}
`
)

// Parsed once at startup, so a broken template fails immediately rather than
// mid-session.
var (
	funcmap = template.FuncMap{
		"join": strings.Join,
		"truncate": func(s int, str string) string {
			if len(str) > s {
				return str[0:s]
			}
			return str
		},
		"green":     green,
		"red":       red,
		"yellow":    yellow,
		"magenta":   magenta,
		"underline": underline,
		"checkMark": checkMark,
		"q2":        q2Plain, // game text, readable (see q2text.go)
		"connected": connectionIndicator,
		"online":    playersOnline,
		"now":       func() int64 { return time.Now().Unix() },
		"ago":       util.TimeAgo,
	}

	helpTmpl   = template.Must(template.New("helpout").Funcs(funcmap).Parse(helpTemplate))
	statusTmpl = template.Must(template.New("statusout").Funcs(funcmap).Parse(statusTemplate))
	searchTmpl = template.Must(template.New("searchout").Funcs(funcmap).Parse(searchTemplate))
	rulesTmpl  = template.Must(template.New("rulesout").Funcs(funcmap).Parse(rulesTemplate))
	whoisTmpl  = template.Must(template.New("whoisout").Funcs(funcmap).Parse(whoisTemplate))
	srvTmpl    = template.Must(template.New("srvout").Funcs(funcmap).Parse(serversTemplate))
)
