package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"google.golang.org/protobuf/encoding/prototext"

	pb "github.com/packetflinger/q2admind/proto"
)

var (
	VPNCache map[string]VPNCacheEntry
)

// Address ranges associated with VPN providers will probably
// not update very frequently. If an IP is not in the cache,
// look it up from the upstream source, then save it for future
// lookups.
type VPNCacheEntry struct {
	ip      string // the IP in question
	vpn     bool   // is this IP from a VPN provider?
	ttl     int64  // unix timestamp when this record is considered stale
	lookups int64  // how many times this IP has been looked up
}

// Load the vpn config proto from disk
func ReadVPNConfig(cfgfile string) (*pb.VPNConfig, error) {
	var cfg *pb.VPNConfig
	contents, err := os.ReadFile(cfgfile)
	if err != nil {
		return nil, fmt.Errorf("unable to open VPN config: %v", err)
	}
	err = prototext.Unmarshal(contents, cfg)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling VPN config: %v", err)
	}
	return cfg, nil
}

func InitVPNCache() {
	VPNCache = make(map[string]VPNCacheEntry)
}

// Check if the supplied IP address is from a VPN service. First look at the cache, then
// lookup via the actual service (and add to the cache)
func IsVPN(ip string, cache map[string]VPNCacheEntry, config *pb.VPNConfig) (bool, error) {
	if config == nil {
		return false, fmt.Errorf("null config")
	}
	if !config.Enabled {
		return false, nil
	}
	r, found := cache[ip]
	if found {
		fmt.Println(ip, "found!")
		if r.vpn && r.ttl > time.Now().Unix() {
			return true, nil
		} else {
			return false, nil
		}
	} else {
		fmt.Println(ip, "not found")
		lookup, err := lookupVPNStatus(ip, config)
		if err != nil {
			return false, err
		}
		entry := VPNCacheEntry{
			ip:      ip,
			vpn:     lookup,
			ttl:     time.Now().Unix() + (86400 * 30), // a month
			lookups: 0,
		}
		cache[ip] = entry
		return lookup, nil
	}
}

func lookupVPNStatus(ip string, config *pb.VPNConfig) (bool, error) {
	if ip == "" {
		return false, fmt.Errorf("empty ip looking up vpn status")
	}
	if config == nil {
		return false, fmt.Errorf("null config looking up vpn status")
	}
	type Results struct {
		Security struct {
			VPN bool `JSON:"vpn"`
		} `JSON:"security"`
	}

	apiClient := http.Client{
		Timeout: time.Second * 2,
	}
	url := fmt.Sprintf(config.GetLookupUrl(), ip, config.GetApiKey())
	res, err := apiClient.Get(url)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return false, err
	}
	var results Results
	err = json.Unmarshal(body, &results)
	if err != nil {
		return false, err
	}

	return results.Security.VPN, nil
}
