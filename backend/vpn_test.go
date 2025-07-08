package backend

import (
	"testing"
	"time"

	pb "github.com/packetflinger/q2admind/proto"
)

func TestLookupVPNStatus(t *testing.T) {
	tests := []struct {
		desc   string
		ip     string
		config *pb.VPNConfig
		want   bool
	}{
		{
			// normally this test is true, but removing my api key...
			desc: "test1",
			ip:   "154.6.80.10",
			config: &pb.VPNConfig{
				Enabled:   true,
				LookupUrl: "https://vpnapi.io/api/%s?key=%s",
				ApiKey:    "yeahright",
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := lookupVPNStatus(tc.ip, tc.config)
			if err != nil {
				t.Error(err)
			}
			if got != tc.want {
				t.Error("lookupVPNStatus() =", got, ", want", tc.want)
			}
		})
	}
}

func TestIsVPN(t *testing.T) {
	tests := []struct {
		desc   string
		ip     string
		cache  map[string]VPNCacheEntry
		config *pb.VPNConfig
		want   bool
	}{
		{
			desc:   "test1",
			ip:     "154.6.80.10",
			config: &pb.VPNConfig{Enabled: true},
			cache: map[string]VPNCacheEntry{
				"154.6.80.10": {
					vpn: true,
					ttl: time.Now().Unix() + 100,
				},
				"8.8.8.8": {
					vpn: false,
					ttl: time.Now().Unix() + 100,
				},
			},
			want: true,
		},
		{
			desc: "test2",
			ip:   "154.6.80.10",
			config: &pb.VPNConfig{
				Enabled:   true,
				LookupUrl: "https://vpnapi.io/api/%s?key=%s",
				ApiKey:    "yeahright",
			},
			cache: map[string]VPNCacheEntry{
				"8.8.8.8": {
					vpn: false,
					ttl: time.Now().Unix() + 100,
				},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := IsVPN(tc.ip, tc.cache, tc.config)
			if err != nil {
				t.Error(err)
			}
			if got != tc.want {
				t.Error("IsVPN() =", got, ", want", tc.want)
			}
		})
	}
}
