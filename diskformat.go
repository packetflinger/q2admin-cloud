// structures for reading and writing to disk
package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

// JSON structure for persistent storage
type ServerFormat struct {
	UUID          string             `json:"UUID"` // match client to server config
	AllowTeleport bool               `json:"AllowTeleport"`
	AllowInvite   bool               `json:"AllowInvite"`
	Enabled       bool               `json:"Enabled"`
	Verified      bool               `json:"Verified"`
	Address       string             `json:"Address"`
	Name          string             `json:"Name"`        // teleport name, must be unique
	Owner         string             `json:"Owner"`       // ID from UserFormat
	Description   string             `json:"Description"` // shows up in teleport
	Contacts      string             `json:"Contacts"`    // for getting ahold of operator
	PublicKey     string             `json:"PublicKey"`   // relative path to file
	Rules         []ClientRuleFormat `json:"Controls"`
}

// Collections of clients
type ServerGroupFormat struct {
	Name    string   `json:"Name"`    // group name
	Owner   string   `json:"Owner"`   // user email/name
	Clients []string `json:"Clients"` // server name
}

// Read a client "object" from disk and into memory.
//
// Called at startup for each client
func (cl *Client) ReadDiskFormat(name string) error {
	sep := os.PathSeparator
	filename := fmt.Sprintf("%s%c%s.json", q2a.config.ClientDirectory, sep, name)
	filedata, err := os.ReadFile(filename)
	if err != nil {
		//log.Println("Problems with", name, "skipping")
		return errors.New("unable to read file")
	}
	sf := ServerFormat{}
	err = json.Unmarshal([]byte(filedata), &sf)
	if err != nil {
		log.Println(err)
		return errors.New("unable to parse data")
	}

	addr := strings.Split(sf.Address, ":")
	if len(addr) == 2 {
		cl.Port, _ = strconv.Atoi(addr[1])
	} else {
		cl.Port = 27910
	}
	cl.IPAddress = addr[0]
	cl.Enabled = sf.Enabled
	cl.Owner = sf.Owner
	cl.Description = sf.Description
	cl.UUID = sf.UUID
	cl.Name = sf.Name
	cl.Verified = sf.Verified

	acls := []ClientRule{}
	for _, c := range sf.Rules {
		acl := ClientRule{}
		acl.Address = c.Address
		for _, ip := range c.Address {
			if !strings.Contains(ip, "/") { // no cidr notation, assuming /32
				ip += "/32"
			}
			_, netbinary, err := net.ParseCIDR(ip)
			if err != nil {
				log.Println("invalid cidr network in rule", c.ID, ip)
				continue
			}
			acl.Network = append(acl.Network, netbinary)
		}
		acl.Hostname = c.Hostname
		acl.Client = c.Client
		acl.Created = c.Created
		acl.Description = c.Description
		acl.Length = c.Length
		acl.Message = c.Message
		acl.Name = c.Name
		acl.Password = c.Password
		acl.StifleLength = c.StifleLength
		acl.Type = c.Type
		acl.UserInfoKey = c.UserInfoKey
		acl.UserinfoVal = c.UserinfoVal
		acls = append(acls, acl)
	}
	cl.Rules = SortRules(acls)
	return nil
}

// Write the current server "object" to disk as JSON
func (cl *Client) WriteDiskFormat() {

	dfrules := []ClientRuleFormat{}
	for _, sc := range cl.Rules {
		c := ClientRuleFormat{}
		c.Type = sc.Type
		c.Address = sc.Address
		c.Name = sc.Name
		c.Client = sc.Client
		c.UserInfoKey = sc.UserInfoKey
		c.UserinfoVal = sc.UserinfoVal
		c.Description = sc.Description
		c.Password = sc.Password
		c.Created = sc.Created
		c.Length = sc.Length
		c.Message = sc.Message
		dfrules = append(dfrules, c)
	}

	d := ServerFormat{
		Enabled:     cl.Enabled,
		Address:     fmt.Sprintf("%s:%d", cl.IPAddress, cl.Port),
		Name:        cl.Name,
		UUID:        cl.UUID,
		Owner:       cl.Owner,
		Description: cl.Description,
		Verified:    cl.Verified,
		Rules:       dfrules,
	}

	filecontents, err := json.MarshalIndent(d, "", " ")
	if err != nil {
		log.Println(err)
	}

	// name property is required, if not found, set random one
	if d.Name == "" {
		d.Name = hex.EncodeToString(RandomBytes(20))
	}
	sep := os.PathSeparator
	filename := fmt.Sprintf("%s%c%s.json", q2a.config.ClientDirectory, sep, d.Name)
	err = os.WriteFile(filename, filecontents, 0644)
	if err != nil {
		log.Println(err)
	}
}
