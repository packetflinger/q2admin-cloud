package frontend

// SearchResult is one player found by a player search: everything recorded
// for one name+IP pair.
type SearchResult struct {
	Name        string
	IP          string
	Hostnames   []string // most recent first
	Versions    []string // clients used, most recent first
	Servers     []string // UUIDs of the servers they played on, most recent first
	VPN         bool     // any visit was from a VPN
	Visits      int
	ConnectTime int64  // the latest visit
	Lookup      string // base64(Name\\ConnectTime), the latest visit's record
}
