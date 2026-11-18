// Template functions for the website: formatting values for display and
// small summaries computed while a page renders.

package backend

import (
	"fmt"
	"html/template"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"github.com/packetflinger/libq2/flags"
	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
)

// userinfoLines formats a rule's userinfo patterns for the rule form.
func userinfoLines(uis []*pb.UserInfo) string {
	var l []string
	for _, ui := range uis {
		l = append(l, ui.GetProperty()+"="+ui.GetValue())
	}
	return strings.Join(l, "\n")
}

// IPAddr is an address split for display: the IP, which gets linked to a
// lookup, and the text around it, like "[" and "]:27901".
type IPAddr struct {
	Before, IP, After string
}

// splitIPAddr splits "192.0.2.1:27901" or "[2001:db8::1]:27901" around the
// IP. A bare IP has nothing around it. IP is empty if addr isn't one of
// these, so nothing gets linked.
func splitIPAddr(addr string) IPAddr {
	if ip, err := netip.ParseAddr(addr); err == nil {
		return IPAddr{IP: ip.String()}
	}
	ap, err := netip.ParseAddrPort(addr)
	if err != nil {
		return IPAddr{After: addr}
	}
	port := fmt.Sprintf(":%d", ap.Port())
	if strings.HasPrefix(addr, "[") {
		return IPAddr{Before: "[", IP: ap.Addr().String(), After: "]" + port}
	}
	return IPAddr{IP: ap.Addr().String(), After: port}
}

// ipLensURL is the IPverse Lens page about ip.
func ipLensURL(ip string) string {
	return "https://lens.ipverse.net/" + url.PathEscape(ip)
}

// slicePercent is v's share of all the slices, like "42%".
func slicePercent(v int, slices []Slice) string {
	total := 0
	for _, s := range slices {
		total += s.Value
	}
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%d%%", (v*100+total/2)/total)
}

// formatAvg formats an average to one decimal place, or "-" when there is
// none (nil).
func formatAvg(v *float64) string {
	if v == nil {
		return "-"
	}
	return strconv.FormatFloat(*v, 'f', 1, 64)
}

// playDuration formats seconds of play like "2h 05m" or "12m".
func playDuration(secs int64) string {
	if secs < 60 {
		return "<1m"
	}
	if secs < 3600 {
		return fmt.Sprintf("%dm", secs/60)
	}
	return fmt.Sprintf("%dh %02dm", secs/3600, secs%3600/60)
}

// playersOnline counts fe's occupied player slots. PlayerCount isn't used
// because it isn't reset when a frontend reconnects.
func playersOnline(fe *frontend.Frontend) int {
	n := 0
	for _, p := range fe.Players {
		if p.ConnectTime > 0 {
			n++
		}
	}
	return n
}

// FrontendSummary is the dashboard's totals across a user's frontends.
type FrontendSummary struct {
	Servers, Online, Players, Rules int
}

// summarize totals fes for the dashboard. Call it from a template rendered by
// executeLocked so the frontends are locked.
func summarize(fes []*frontend.Frontend) FrontendSummary {
	s := FrontendSummary{Servers: len(fes)}
	for _, fe := range fes {
		if fe.Connected && fe.Trusted {
			s.Online++
			s.Players += playersOnline(fe)
		}
		s.Rules += len(fe.Rules)
	}
	return s
}

// SoftwareVersions is what a frontend runs, for the dashboard's versions
// table. Empty fields are unknown.
type SoftwareVersions struct {
	Engine   string // the server binary, like "q2proded r3732~7b63f040 Sep 14 2026 Linux x86_64"
	Q2Admin  string // like "r1306~c96d8e1"
	Game     string // the game library q2admin wraps, like "baseq2"
	GameDate string // the game library's build date
	GameRev  string // the game library's revision number, only OpenTDM sets it
}

// softwareVersions reads fe's versions from the serverinfo fetched after it
// connected, which stays around after it disconnects. q2admin's own version
// falls back to the build number from its greeting. GameRev is only set when
// "revision" is a number. Call it from a template
// rendered by executeLocked so fe is locked.
func softwareVersions(fe *frontend.Frontend) SoftwareVersions {
	v := SoftwareVersions{
		Engine:   fe.ServerVars["version"],
		Q2Admin:  fe.ServerVars["q2admin"],
		Game:     fe.ServerVars["gamename"],
		GameDate: fe.ServerVars["gamedate"],
	}
	if rev := fe.ServerVars["revision"]; rev != "" {
		if _, err := strconv.ParseUint(rev, 10, 64); err == nil {
			v.GameRev = rev
		}
	}
	if v.Q2Admin == "" && fe.Version > 0 {
		v.Q2Admin = fmt.Sprintf("r%d", fe.Version)
	}
	return v
}

// Given a boolean, return html for a green "yes" or red "no"
func boolToYesNo(val bool) template.HTML {
	if val {
		return template.HTML(fmt.Sprintf("<span class=%q>yes</span>", "text-success"))
	}
	return template.HTML(fmt.Sprintf("<span class=%q>no</span>", "text-danger"))
}

// Given a boolean, return a green checkbox emoji or red X
func boolToEmoji(val bool) template.HTML {
	if val {
		return template.HTML("&#x2705;") // green checkmark
	}
	return template.HTML("&#x274C;") // red x
}

// for translating boolean value from struct to an HTML checkbox value
func boolToChecked(val bool) string {
	if val {
		return "checked"
	}
	return ""
}

// convert a string representation of dmflags bitmask and show the values.
func dmflags(val string) string {
	fl, err := strconv.Atoi(val)
	if err != nil {
		return "invalid"
	}
	return flags.ToString(fl)
}
