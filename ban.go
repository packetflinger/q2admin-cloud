package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

type Ban struct {
	address     string
	condition   string
	value       string
	description string
}

const (
	NotBanned = iota // no ban matches
	Banned           // explicity banned
	Allowed          // a ban matches, but criteria allows
)

var globalbans []Ban

/**
 * Parse ban files loading records into memory.
 * Runs at startup
 */
func LoadGlobalBans() {
	banfile := "bans/global.csv"
	log.Printf("Loading global banlist from %s\n", banfile)
	bandata, err := os.ReadFile(banfile)
	if err != nil {
		log.Println("Problems loading banlist: ", err)
		return
	}

	r := csv.NewReader(strings.NewReader(string(bandata)))
	for {
		r.Comment = '#'
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		ban := Ban{
			address:     record[0],
			description: record[2],
		}

		globalbans = append(globalbans, ban)
	}
}

/**
 * Load gameserver local banlist
 * Happens after gameserver connects and authenticates
 */
func LoadBans(cl *Client) {
	/*
		banfile := fmt.Sprintf("bans/%s.csv", cl.Name)

		bandata, err := os.ReadFile(banfile)
		if err != nil {
			log.Printf("[%s] problems loading banlist: %s\n", cl.Name, err)
			return
		}

		r := csv.NewReader(strings.NewReader(string(bandata)))
		for {
			r.Comment = '#'
			record, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}

			ban := Ban{
				address:     record[0],
				description: record[2],
			}

			cl.Bans = append(srv.Bans, ban)
		}

		log.Printf("[%s] banlist loaded: %s\n", srv.Name, banfile)
	*/
}

/**
 *
 */
func CheckForBan(banlist *[]Ban, ip string) (int, string) {
	ipaddr, _, err := net.ParseCIDR(fmt.Sprintf("%s/32", ip))
	if err != nil {
		log.Println("Converting IP: ", err)
		return NotBanned, ""
	}

	for _, ban := range *banlist {
		_, net, err := net.ParseCIDR(ban.address)
		if err != nil {
			log.Println("Ban lookup: ", err)
			continue
		}

		if net.Contains(ipaddr) {
			log.Printf("%s is BANNED\n", ip)
			return Banned, ban.description
		}
	}

	return NotBanned, ""
}
