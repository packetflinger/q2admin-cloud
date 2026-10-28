package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"

	"github.com/packetflinger/q2admind/frontend"
)

const (
	searchPlayers = 200 // results returned
	// shortcut: a player with more name/IP/client/server combinations than
	// fit in this many rows loses their oldest clients and servers from the
	// lists; raise it if that shows up.
	searchCombos = 2000
)

// SearchPlayers finds players on these servers (UUIDs) whose userinfo (which
// holds their name and IP), hostname, client version or player cookie contains
// what. There's one result per name+IP pair, with the clients, hostnames and
// servers they used, most recently seen first.
func SearchPlayers(ctx context.Context, what string, servers []string) ([]frontend.SearchResult, error) {
	if len(servers) == 0 {
		return nil, nil
	}
	// filter to the servers before grouping, so the lists and visit counts
	// only cover what the searcher may see
	qry := `
	SELECT p.name, p.ip, COALESCE(p.hostname, ''), COALESCE(p.version, ''), f.uuid,
		MAX(p.vpn), MAX(p.time), COUNT(*)
	FROM player AS p
	JOIN frontend AS f ON f.name = p.server -- player.server holds the frontend name
	WHERE (p.userinfo LIKE ? OR p.hostname LIKE ? OR p.version LIKE ? OR p.cookie LIKE ?)
		AND f.uuid IN (` + strings.TrimSuffix(strings.Repeat("?,", len(servers)), ",") + `)
	GROUP BY p.name, p.ip, p.hostname, p.version, f.uuid
	ORDER BY MAX(p.time) DESC
	LIMIT ?`
	like := "%" + what + "%"
	args := []any{like, like, like, like}
	for _, s := range servers {
		args = append(args, s)
	}
	rows, err := db.Handle.QueryContext(ctx, qry, append(args, searchCombos)...)
	if err != nil {
		return nil, fmt.Errorf("search query: %v", err)
	}
	defer rows.Close()

	var results []*frontend.SearchResult
	byPlayer := map[[2]string]*frontend.SearchResult{}
	for rows.Next() {
		var name, ip, hostname, version, server string
		var vpn bool
		var seen int64
		var visits int
		if err := rows.Scan(&name, &ip, &hostname, &version, &server, &vpn, &seen, &visits); err != nil {
			return nil, fmt.Errorf("scanning search results: %v", err)
		}
		r := byPlayer[[2]string{name, ip}]
		if r == nil {
			if len(results) == searchPlayers {
				continue // only adding to players we already have
			}
			// rows come newest first, so the first is the player's latest visit
			r = &frontend.SearchResult{Name: name, IP: ip, ConnectTime: seen}
			byPlayer[[2]string{name, ip}] = r
			results = append(results, r)
		}
		r.Visits += visits
		r.VPN = r.VPN || vpn
		r.Hostnames = appendNew(r.Hostnames, hostname)
		r.Versions = appendNew(r.Versions, version)
		r.Servers = appendNew(r.Servers, server)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]frontend.SearchResult, len(results))
	for i, r := range results {
		r.Lookup = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s\\%d", r.Name, r.ConnectTime)))
		out[i] = *r
	}
	return out, nil
}

// appendNew adds s to list unless it's empty or already there.
func appendNew(list []string, s string) []string {
	if s == "" || slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}
