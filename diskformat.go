// structures for reading and writing to disk
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// Collections of clients
type ServerGroupFormat struct {
	Name    string   `json:"Name"`    // group name
	Owner   string   `json:"Owner"`   // user email/name
	Clients []string `json:"Clients"` // server name
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

	d := ClientDiskFormat{
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
