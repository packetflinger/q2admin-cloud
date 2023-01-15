// structures for reading and writing to disk
package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

type UserFormat struct {
	ID          string `json:"ID"` // uuid
	Email       string `json:"Email"`
	Description string `json:"Description"`
	LoginCount  int    `json:"LoginCount"`
	LastLogin   int    `json:"LastLogin"` // unix timestamp
}

// JSON structure for persistent storage
type ServerFormat struct {
	UUID          string                `json:"UUID"` // match client to server config
	AllowTeleport bool                  `json:"AllowTeleport"`
	AllowInvite   bool                  `json:"AllowInvite"`
	Enabled       bool                  `json:"Enabled"`
	Verified      bool                  `json:"Verified"`
	Address       string                `json:"Address"`
	Name          string                `json:"Name"`        // teleport name, must be unique
	Owner         string                `json:"Owner"`       // ID from UserFormat
	Description   string                `json:"Description"` // shows up in teleport
	Contacts      string                `json:"Contacts"`    // for getting ahold of operator
	PublicKey     string                `json:"PublicKey"`   // relative path to file
	Controls      []ServerControlFormat `json:"Controls"`
}

// for bans, mutes, onjoin msgs
type ServerControlFormat struct {
	Type         string   `json:"Type"` // ["ban","mute","stifle","msg"]
	Address      string   `json:"Address"`
	Name         []string `json:"Name"`        // optional
	Client       []string `json:"Client"`      // optional
	Message      string   `json:"Message"`     // shown to user
	UserInfoKey  []string `json:"UserInfoKey"` // optional
	UserinfoVal  []string `json:"UserInfoVal"` // optional
	Description  string   `json:"Description"`
	Insensitive  bool     `json:"Insensitive"`  // case insensitive?
	Exact        bool     `json:"Exact"`        // must match exactly, not just contains
	Password     string   `json:"Password"`     // override userinfo password
	StifleLength int      `json:"StifleLength"` // seconds
	Created      int64    `json:"Created"`      // unix timestamp
	Length       int64    `json:"Length"`       // seconds after created before expires
}

type ServerGroupFormat struct {
	Name    string   `json:"Name"`    // group name
	Owner   string   `json:"Owner"`   // user email/name
	Servers []string `json:"Servers"` // server name
}

// read a server "object" from disk and into memory
func (cl *Client) ReadDiskFormat(name string) error {
	sep := os.PathSeparator
	filename := fmt.Sprintf("%s%c%s.json", config.ServerDirectory, sep, name)
	filedata, err := os.ReadFile(filename)
	if err != nil {
		log.Println("Problems with", name, "skipping")
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

	controls := []ClientControls{}
	for _, c := range sf.Controls {
		control := ClientControls{}
		control.Address = c.Address
		control.Client = c.Client
		control.Created = c.Created
		control.Description = c.Description
		control.Length = c.Length
		control.Message = c.Message
		control.Name = c.Name
		control.Password = c.Password
		control.StifleLength = c.StifleLength
		control.Type = c.Type
		control.UserInfoKey = c.UserInfoKey
		control.UserinfoVal = c.UserinfoVal
		controls = append(controls, control)
	}
	cl.Controls = controls
	return nil
}

// Write the current server "object" to disk as JSON
func (cl *Client) WriteDiskFormat() {

	dfcontrols := []ServerControlFormat{}
	for _, sc := range cl.Controls {
		c := ServerControlFormat{}
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
		dfcontrols = append(dfcontrols, c)
	}

	d := ServerFormat{
		Enabled:     cl.Enabled,
		Address:     fmt.Sprintf("%s:%d", cl.IPAddress, cl.Port),
		Name:        cl.Name,
		UUID:        cl.UUID,
		Owner:       cl.Owner,
		Description: cl.Description,
		Verified:    cl.Verified,
		Controls:    dfcontrols,
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
	filename := fmt.Sprintf("%s%c%s.json", config.ServerDirectory, sep, d.Name)
	err = os.WriteFile(filename, filecontents, 0644)
	if err != nil {
		log.Println(err)
	}
}
