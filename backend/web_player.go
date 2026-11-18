// Website page for a connected player and the actions on them.

package backend

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	pb "github.com/packetflinger/q2admind/proto"
)

// PlayerViewHandler shows a connected player and the controls for them.
func PlayerViewHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	data := PageResponse{SessionUser: user, Frontend: fe, Notice: notices[r.URL.Query().Get("notice")]}
	data.NavHighlight.Servers = "active"
	if id, err := strconv.Atoi(mux.Vars(r)["ClientNum"]); err == nil {
		fe.Mu.Lock()
		data.Player, _ = fe.FindPlayer(id) // nil renders "not connected"
		fe.Mu.Unlock()
	}
	data.Head.Title = "Player | CloudAdmin"
	tmpl, err := parsePage("player-view.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, "player-view", data, fe); err != nil {
		log.Println(err)
	}
}

// PlayerActionHandler messages, kicks, mutes or stifles a connected player.
func PlayerActionHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	id, _ := strconv.Atoi(mux.Vars(r)["ClientNum"])
	// newlines would break the in-game text, and it's a short chat line
	msg := strings.Join(strings.Fields(r.PostFormValue("msg")), " ")
	if len(msg) > 150 {
		msg = msg[:150]
	}
	secs, secsErr := strconv.Atoi(r.PostFormValue("seconds"))

	fe.Mu.Lock()
	defer fe.Mu.Unlock()
	back := path.Join("/sv", fe.UUID, fe.Name, "player", strconv.Itoa(id))
	p, err := fe.FindPlayer(id)
	if err != nil || !(fe.Connected && fe.Trusted) {
		http.Redirect(w, r, back+"?notice=noplayer", http.StatusSeeOther)
		return
	}
	notice := ""
	switch r.PostFormValue("action") {
	case "message":
		if msg == "" {
			notice = "badinput"
			break
		}
		SayPlayer(fe, p, PRINT_CHAT, msg)
		notice = "sent"
	case "kick":
		KickPlayer(fe, p, msg)
		SendMessages(fe)
		http.Redirect(w, r, path.Join("/sv", fe.UUID, fe.Name)+"?notice=kicked#players", http.StatusSeeOther)
		return
	case "ban":
		rule := &pb.Rule{
			Uuid:        uuid.NewString(),
			Type:        pb.RuleType_BAN,
			Scope:       "local",
			Address:     []string{p.IP + "/32"},
			Description: []string{fmt.Sprintf("%s banned by %s from the website", p.Name, user.GetEmail())},
		}
		if msg != "" {
			rule.Message = []string{msg}
		}
		if net.ParseIP(p.IP).To4() == nil {
			rule.Address = []string{p.IP + "/128"}
		}
		if err := fe.InsertRule(rule); err != nil {
			log.Println(err)
			http.Error(w, "500 - unable to save the ban rule", http.StatusInternalServerError)
			return
		}
		fe.Rules = append(fe.Rules, rule)
		KickPlayer(fe, p, msg)
		SendMessages(fe)
		http.Redirect(w, r, path.Join("/sv", fe.UUID, fe.Name)+"?notice=banned#players", http.StatusSeeOther)
		return
	case "blackhole":
		ip := net.ParseIP(p.IP)
		if ip == nil { // it goes out as a console command
			notice = "badinput"
			break
		}
		ConsoleCommand(fe, "addblackhole "+ip.String())
		notice = "blackholed"
	case "mute":
		if secsErr != nil || secs < 0 || secs > 86400 {
			notice = "badinput"
			break
		}
		MutePlayer(fe, p, secs) // 0 is permanent
		notice = "muted"
	case "stifle":
		if secsErr != nil || secs < 1 || secs > StifleMax {
			notice = "badinput"
			break
		}
		StiflePlayer(fe, p, secs)
		notice = "stifled"
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	SendMessages(fe)
	http.Redirect(w, r, back+"?notice="+notice, http.StatusSeeOther)
}
