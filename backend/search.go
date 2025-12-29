package backend

import (
	"context"
	"encoding/base64"
	"fmt"
)

type SearchResult struct {
	Name        string
	IP          string
	Hostname    string
	VPN         bool
	Version     string
	ConnectTime int64
	ServerUUID  string
	Lookup      string // base64(Name\\ConnectTime)
}

// A search was requested from the website, this finds the actual data in the
// database. The fields searched will be:
//   - player userinfo (contains IP and name)
//   - player hostname
//   - player client version
//   - player cookie
func (b *Backend) Search(ctx context.Context, what string) ([]SearchResult, error) {
	var results []SearchResult
	qry := `
	SELECT 
		p.name,
		p.ip,
		p.hostname,
		p.vpn,
		p.version,
		p.time,
		f.uuid AS server
	FROM player AS p
	JOIN frontend AS f ON f.id = p.server_id
	WHERE true
		AND (
			p.userinfo LIKE ? OR
			p.hostname LIKE ? OR
			p.version LIKE ? OR
			p.cookie LIKE ?
		)
	GROUP BY p.name, p.ip, p.hostname, p.version, f.uuid
	ORDER BY p.time DESC;
	`
	what = fmt.Sprintf("%%%s%%", what)
	res, err := db.Handle.QueryContext(ctx, qry, what, what, what, what)
	if err != nil {
		return results, fmt.Errorf("search query: %v", err)
	}
	defer res.Close()
	for res.Next() {
		var r SearchResult
		err := res.Scan(&r.Name, &r.IP, &r.Hostname, &r.VPN, &r.Version, &r.ConnectTime, &r.ServerUUID)
		if err != nil {
			return results, fmt.Errorf("scanning search results: %v", err)
		}
		r.Lookup = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s\\%d", r.Name, r.ConnectTime)))
		results = append(results, r)
	}
	return results, nil
}
