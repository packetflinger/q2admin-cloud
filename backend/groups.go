// Server groups let a user collect their frontends under any name they like
// (by mod, location, zone...) and send one console command to all of them.
// A frontend can be in any number of groups. Groups live in the database and
// are read when needed: there are few of them and they're rarely used.
package backend

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/packetflinger/q2admind/frontend"
)

// ServerGroup is one of a user's groups of frontends.
type ServerGroup struct {
	ID      int64
	Name    string
	Members []*frontend.Frontend // in Frontends() order
}

// MyGroups returns owner's groups, sorted by name.
func MyGroups(owner string) ([]*ServerGroup, error) {
	rows, err := db.Handle.Query(`
		SELECT g.id, g.name, COALESCE(m.frontend_id, 0)
		FROM server_group AS g
		LEFT JOIN server_group_member AS m ON m.group_id = g.id
		WHERE g.owner = ?
		ORDER BY g.name`, owner)
	if err != nil {
		return nil, fmt.Errorf("error selecting groups: %v", err)
	}
	defer rows.Close()
	var groups []*ServerGroup
	memberIDs := map[*ServerGroup]map[int]bool{}
	for rows.Next() {
		var id int64
		var name string
		var feID int
		if err := rows.Scan(&id, &name, &feID); err != nil {
			return nil, fmt.Errorf("error scanning groups: %v", err)
		}
		if len(groups) == 0 || groups[len(groups)-1].ID != id {
			groups = append(groups, &ServerGroup{ID: id, Name: name})
			memberIDs[groups[len(groups)-1]] = map[int]bool{}
		}
		memberIDs[groups[len(groups)-1]][feID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, fe := range be.Frontends() {
		for _, g := range groups {
			if memberIDs[g][fe.ID] { // ID is set before a frontend is listed and never changes
				g.Members = append(g.Members, fe)
			}
		}
	}
	return groups, nil
}

// MyGroup returns owner's group with this name.
func MyGroup(owner, name string) (*ServerGroup, error) {
	groups, err := MyGroups(owner)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		if g.Name == name {
			return g, nil
		}
	}
	return nil, fmt.Errorf("no group named %q", name)
}

// CreateGroup adds an empty group for owner. Names follow the server name
// rules so they work as SSH arguments and in URLs.
func CreateGroup(owner, name string) error {
	if !serverNameRE.MatchString(name) {
		return errors.New("group names are 1-32 lowercase letters, numbers or dashes")
	}
	_, err := db.Handle.Exec("INSERT INTO server_group (owner, name) VALUES (?, ?)", owner, name)
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return fmt.Errorf("you already have a group named %q", name)
	}
	if err != nil {
		return fmt.Errorf("error creating group: %v", err)
	}
	return nil
}

// DeleteGroup removes one of owner's groups. Its servers aren't affected.
func DeleteGroup(owner, name string) error {
	g, err := MyGroup(owner, name)
	if err != nil {
		return err
	}
	tx, err := db.Handle.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM server_group_member WHERE group_id = ?", g.ID); err != nil {
		return fmt.Errorf("error deleting group members: %v", err)
	}
	if _, err := tx.Exec("DELETE FROM server_group WHERE id = ?", g.ID); err != nil {
		return fmt.Errorf("error deleting group: %v", err)
	}
	return tx.Commit()
}

// SetGroupMember adds fe to, or removes it from, one of owner's groups. Only
// servers the user owns or has been delegated can be added.
func SetGroupMember(owner, name string, fe *frontend.Frontend, member bool) error {
	g, err := MyGroup(owner, name)
	if err != nil {
		return err
	}
	fe.Mu.Lock()
	access, feName, feID := fe.CanAccess(owner), fe.Name, fe.ID
	fe.Mu.Unlock()
	if !access {
		return fmt.Errorf("you don't have a server named %q", feName)
	}
	qry := "DELETE FROM server_group_member WHERE group_id = ? AND frontend_id = ?"
	if member {
		qry = "INSERT OR IGNORE INTO server_group_member (group_id, frontend_id) VALUES (?, ?)"
	}
	if _, err := db.Handle.Exec(qry, g.ID, feID); err != nil {
		return fmt.Errorf("error updating group %q: %v", name, err)
	}
	return nil
}

// SendGroupCommand runs cmd on the console of every connected server in one
// of owner's groups, as the web console does for one server. who is recorded
// in each server's log and shown to its console watchers.
func SendGroupCommand(owner, name, cmd, who string) (sent, offline int, err error) {
	if err := validConsoleCommand(cmd); err != nil {
		return 0, 0, err
	}
	g, err := MyGroup(owner, name)
	if err != nil {
		return 0, 0, err
	}
	for _, fe := range g.Members { // one lock at a time, never nested
		fe.Mu.Lock()
		if fe.Connected && fe.Trusted {
			ConsoleCommand(fe, cmd)
			SendMessages(fe)
			fe.Log.Printf("group %s [%s]: %s\n", name, who, cmd)
			fe.ConsoleBroadcast(fmt.Sprintf("[%s via group %s] > %s", who, name, cmd))
			sent++
		} else {
			offline++
		}
		fe.Mu.Unlock()
	}
	return sent, offline, nil
}

// sshGroupCommand runs the SSH "group"/"groups" commands for owner and
// returns what to print.
func sshGroupCommand(owner string, c CmdArgs) string {
	usage := `Usage:
  groups                          list your groups
  group show <group>              list a group's servers
  group create <group>            make an empty group
  group delete <group>            delete a group (its servers are kept)
  group add <group> <server>      put a server in a group
  group remove <group> <server>   take a server out of a group
  group cmd <group> <command>     run a console command on every server in a group`
	sub := ""
	if c.argc > 0 {
		sub = strings.ToLower(c.argv[0])
	}
	if c.command == "groups" || (c.command == "group" && c.argc == 0) {
		sub = "list"
	}
	need := map[string]int{"list": 0, "show": 2, "create": 2, "delete": 2, "add": 3, "remove": 3, "cmd": 3}
	n, known := need[sub]
	if !known || c.argc < n {
		return usage
	}
	var err error
	switch sub {
	case "list":
		groups, err := MyGroups(owner)
		if err != nil {
			return err.Error()
		}
		if len(groups) == 0 {
			return "You don't have any groups yet. Make one with: group create <name>"
		}
		var out strings.Builder
		for _, g := range groups {
			fmt.Fprintf(&out, "%-20s %d server(s)\n", g.Name, len(g.Members))
		}
		return out.String()
	case "show":
		g, err := MyGroup(owner, c.argv[1])
		if err != nil {
			return err.Error()
		}
		if len(g.Members) == 0 {
			return fmt.Sprintf("%s has no servers. Add one with: group add %s <server>", g.Name, g.Name)
		}
		var out strings.Builder
		for _, fe := range g.Members {
			fe.Mu.Lock()
			fmt.Fprintf(&out, "%-20s %s\n", fe.Name, connectionIndicator(fe))
			fe.Mu.Unlock()
		}
		return out.String()
	case "create":
		err = CreateGroup(owner, c.argv[1])
	case "delete":
		err = DeleteGroup(owner, c.argv[1])
	case "add", "remove":
		fe, findErr := be.FindFrontendByName(c.argv[2])
		if findErr != nil {
			return fmt.Sprintf("you don't have a server named %q", c.argv[2])
		}
		err = SetGroupMember(owner, c.argv[1], fe, sub == "add")
	case "cmd":
		cmd := strings.Join(c.argv[2:], " ")
		sent, offline, err := SendGroupCommand(owner, c.argv[1], cmd, owner)
		if err != nil {
			return err.Error()
		}
		return fmt.Sprintf("sent to %d server(s), %d offline", sent, offline)
	}
	if err != nil {
		return err.Error()
	}
	return "ok"
}

// GroupsHandler lists the user's groups (GET) or creates one (POST).
func GroupsHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	data := PageResponse{SessionUser: user, Notice: notices[r.URL.Query().Get("notice")]}
	data.Head.Title = "Groups | CloudAdmin"
	data.NavHighlight.Groups = "active"
	if r.Method == http.MethodPost {
		name := strings.TrimSpace(r.PostFormValue("name"))
		if err := CreateGroup(user.GetEmail(), name); err != nil {
			data.FormError, data.Form = err.Error(), r.PostForm
		} else {
			http.Redirect(w, r, "/groups/"+name+"?notice=groupcreated", http.StatusSeeOther)
			return
		}
	}
	if data.Groups, err = MyGroups(user.GetEmail()); err != nil {
		log.Println(err)
		http.Error(w, "500 - unable to load groups", http.StatusInternalServerError)
		return
	}
	data.Frontends = FrontendsByIdentity(user.GetEmail()) // every member is one of these
	renderGroupPage(w, "groups.tmpl", "groups", data)
}

// GroupViewHandler shows one group with its servers and controls.
func GroupViewHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	g, err := MyGroup(user.GetEmail(), mux.Vars(r)["group"])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	data := PageResponse{SessionUser: user, Group: g, Notice: notices[q.Get("notice")]}
	if q.Has("sent") { // counts only, so nothing user-supplied is shown
		sent, _ := strconv.Atoi(q.Get("sent"))
		offline, _ := strconv.Atoi(q.Get("offline"))
		data.Notice = &Notice{"success", fmt.Sprintf("Command sent to %d server(s). %d offline server(s) skipped.", sent, offline)}
	}
	data.Head.Title = g.Name + " | CloudAdmin"
	data.NavHighlight.Groups = "active"
	data.Frontends = FrontendsByIdentity(user.GetEmail()) // for the add-server list
	renderGroupPage(w, "group-view.tmpl", "group", data)
}

// GroupActionHandler deletes a group, adds or removes a server, or sends a
// command to the group.
func GroupActionHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	owner, name := user.GetEmail(), mux.Vars(r)["group"]
	back := "/groups/" + name
	switch r.PostFormValue("action") {
	case "delete":
		err = DeleteGroup(owner, name)
		back = "/groups?notice=groupdeleted"
	case "add", "remove":
		fe, findErr := be.FindFrontend(r.PostFormValue("server"))
		if findErr != nil {
			http.NotFound(w, r)
			return
		}
		err = SetGroupMember(owner, name, fe, r.PostFormValue("action") == "add")
		back += "?notice=group" + r.PostFormValue("action")
	case "command":
		sent, offline, cmdErr := SendGroupCommand(owner, name, strings.TrimSpace(r.PostFormValue("cmd")), owner)
		err = cmdErr
		back += fmt.Sprintf("?sent=%d&offline=%d", sent, offline)
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func renderGroupPage(w http.ResponseWriter, file, name string, data PageResponse) {
	tmpl, err := parsePage(file)
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, name, data, data.Frontends...); err != nil {
		log.Println(err)
	}
}
