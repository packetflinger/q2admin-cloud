// Website pages for one server: its overview, adding, editing and deleting
// it, its keys and ID, map changes and who it's shared with.

package backend

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/packetflinger/q2admind/crypto"
	"github.com/packetflinger/q2admind/frontend"

	pb "github.com/packetflinger/q2admind/proto"
)

var serverNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// validateNewServer checks the add-server form. The name ends up in URLs and
// in a directory path, so it's restricted to lowercase letters, digits and
// dashes (spaces become dashes). An empty port means the Quake 2 default.
func validateNewServer(name, address, port string) (string, int, error) {
	name = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), " ", "-")
	if !serverNameRE.MatchString(name) {
		return "", 0, errors.New("name must be 1-32 letters, numbers or dashes")
	}
	address = strings.TrimSpace(address)
	if net.ParseIP(address) == nil && (address == "" || strings.ContainsAny(address, " \t/:") || len(address) > 253) {
		return "", 0, errors.New("address must be an IP address or hostname, without a port")
	}
	if port == "" {
		return name, 27910, nil
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return "", 0, errors.New("port must be a number from 1 to 65535")
	}
	return name, p, nil
}

// Displays info page for a particular client
func WebsiteHandlerServerView(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	renderServerView(w, user, fe, notices[r.URL.Query().Get("notice")], "", r.URL.Query().Get("span"))
}

// renderServerView shows fe's page, optionally with a notice from a previous
// action or the reason a settings change was rejected.
func renderServerView(w http.ResponseWriter, user *pb.User, fe *frontend.Frontend, notice *Notice, formError, span string) {
	data := PageResponse{SessionUser: user, Frontend: fe, Notice: notice, FormError: formError}
	fe.Mu.Lock()
	data.CanEdit = strings.EqualFold(fe.Owner, user.GetEmail()) // delegates can't change settings
	fe.Mu.Unlock()
	var err error
	if data.Stats, err = loadStats([]*frontend.Frontend{fe}, span, false); err != nil {
		log.Println(err)
		http.Error(w, "500 - unable to load stats", http.StatusInternalServerError)
		return
	}
	if data.Stats.Commands, err = loadCommandStats(data.Stats, fe); err != nil {
		log.Println(err)
		http.Error(w, "500 - unable to load stats", http.StatusInternalServerError)
		return
	}
	if data.Signals, err = loadSignals([]*frontend.Frontend{fe}, "", signalsShown); err != nil {
		log.Println(err)
		http.Error(w, "500 - unable to load signals", http.StatusInternalServerError)
		return
	}
	_, length := spanFor(data.Stats.Span)
	if data.Connection, err = loadConnectionHistory(fe, time.Now().Add(-length).Unix()); err != nil {
		log.Println(err)
		http.Error(w, "500 - unable to load connection history", http.StatusInternalServerError)
		return
	}
	fe.Mu.Lock()
	data.Head.Title = fe.Name + " | CloudAdmin"
	fe.Mu.Unlock()
	data.NavHighlight.Servers = "active"
	tmpl, err := parsePage("server-view.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, "server-view", data, fe); err != nil {
		log.Println(err)
	}
}

var mapNameRE = regexp.MustCompile(`^[A-Za-z0-9_/-]{1,64}$`)

// DelegateHandler lets a server's owner give a registered user access to it
// (action=add) or take that access away (action=remove). Delegates can do
// everything except change the server's settings.
func DelegateHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, true)
	if fe == nil {
		return
	}
	email := strings.TrimSpace(r.PostFormValue("email"))
	fe.Mu.Lock()
	defer fe.Mu.Unlock()
	notice := ""
	switch r.PostFormValue("action") {
	case "add":
		u, _ := be.GetUserByEmail(email)
		switch {
		case u == nil:
			notice = "nouser"
		case strings.EqualFold(u.GetEmail(), fe.Owner):
			notice = "selfdelegate"
		case fe.CanAccess(u.GetEmail()):
			notice = "delegated" // already has it
		default:
			if err := addDelegate(fe, u.GetEmail()); err != nil {
				log.Println(err)
				http.Error(w, "500 - unable to save", http.StatusInternalServerError)
				return
			}
			notice = "delegated"
		}
	case "remove":
		for _, d := range fe.Delegates {
			if !strings.EqualFold(d, email) {
				continue
			}
			if _, err := db.Handle.Exec("DELETE FROM frontend_delegate WHERE frontend_id = ? AND email = ?", fe.ID, d); err != nil {
				log.Printf("error removing %q's delegate %q: %v\n", fe.Name, d, err)
				http.Error(w, "500 - unable to save", http.StatusInternalServerError)
				return
			}
			fe.Delegates = slices.DeleteFunc(slices.Clone(fe.Delegates), func(x string) bool { return x == d })
		}
		notice = "undelegated"
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, path.Join("/sv", fe.UUID, fe.Name)+"?notice="+notice+"#settings", http.StatusSeeOther)
}

// addDelegate shares fe with a registered user who doesn't already have
// access. Call it holding fe.Mu.
func addDelegate(fe *frontend.Frontend, email string) error {
	if _, err := db.Handle.Exec("INSERT INTO frontend_delegate (frontend_id, email) VALUES (?, ?)", fe.ID, email); err != nil {
		return fmt.Errorf("error delegating %q to %q: %v", fe.Name, email, err)
	}
	fe.Delegates = append(slices.Clone(fe.Delegates), email)
	return nil
}

// ChangeMapHandler tells a connected frontend to switch maps.
func ChangeMapHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, false)
	if fe == nil {
		return
	}
	mapname := strings.TrimSpace(r.PostFormValue("map"))
	fe.Mu.Lock()
	defer fe.Mu.Unlock()
	notice := "map"
	switch {
	case !mapNameRE.MatchString(mapname): // it's sent as a console command
		notice = "badmap"
	case !(fe.Connected && fe.Trusted):
		notice = "offline"
	default:
		ConsoleCommand(fe, "gamemap "+mapname)
		SendMessages(fe)
	}
	http.Redirect(w, r, path.Join("/sv", fe.UUID, fe.Name)+"?notice="+notice, http.StatusSeeOther)
}

func WebAddServer(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}

	data := PageResponse{SessionUser: user}
	data.Head.Title = "Add Server | CloudAdmin"
	data.NavHighlight.Servers = "active"

	// The add button was pressed to submit the form
	if r.PostFormValue("AddButton") != "" {
		name, port, err := validateNewServer(r.PostFormValue("NewServerName"), r.PostFormValue("NewServerAddress"), r.PostFormValue("NewServerPort"))
		if err == nil {
			if _, dupe := be.FindFrontendByName(name); dupe == nil {
				err = fmt.Errorf("a server named %q already exists", name)
			}
		}
		if err == nil {
			f := &frontend.Frontend{
				IPAddress: strings.TrimSpace(r.PostFormValue("NewServerAddress")),
				Name:      name,
				Owner:     user.Email,
				Path:      path.Join(be.config.ClientDirectory, name),
				Port:      port,
				UUID:      uuid.NewString(),
			}
			if err = be.CreateFrontend(f); err == nil {
				http.Redirect(w, r, path.Join("/sv", f.UUID, f.Name), http.StatusSeeOther)
				return
			}
			log.Printf("error creating frontend %q: %v\n", name, err)
			err = errors.New("unable to save the server, try again")
		}
		data.FormError = err.Error()
		data.Form = r.PostForm
	}

	tmpl, e := parsePage("server-add.tmpl")

	if e != nil {
		log.Println(e)
	} else {
		err = tmpl.ExecuteTemplate(w, "server-add", data)
		if err != nil {
			log.Println(err)
		}
	}
}

// Handler to delete a user's frontend
func WebDelServer(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe, err := be.FindFrontend(mux.Vars(r)["ServerUUID"])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	fe.Mu.Lock()
	owner, name, protected := fe.Owner, fe.Name, fe.DeleteProtect
	fe.Mu.Unlock()
	if owner != user.Email {
		http.Error(w, "403 - not owner", http.StatusForbidden)
		be.Logf(LogLevelInfo, "Non-owner %q attempt to delete %q", user.Email, name)
		return
	}
	if protected {
		http.Error(w, "403 - delete protection is on, turn it off in settings first", http.StatusForbidden)
		return
	}
	err = be.DeleteFrontend(name)
	if err != nil {
		http.Error(w, "500 - error deleting frontend", http.StatusInternalServerError)
		be.Logf(LogLevelInfo, "%s", err)
		return
	}
	http.Redirect(w, r, "/my-servers", http.StatusSeeOther)
}

// PublicKeyHandler serves CloudAdmin's public key, which Quake 2 server admins
// download for their q2admin config. It's encoded from the key loaded at
// startup, so it always matches the private key in use.
func PublicKeyHandler(w http.ResponseWriter, r *http.Request) {
	der, err := x509.MarshalPKIXPublicKey(be.publicKey)
	if err != nil {
		log.Println("error encoding the public key:", err)
		http.Error(w, "500 - public key unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", `attachment; filename="public.pem"`)
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func ChangeUUIDHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	currentUUID := mux.Vars(r)["ServerUUID"]
	f, err := be.FindFrontend(currentUUID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.Owner != user.Email {
		http.Error(w, "403 - unauthorized", http.StatusForbidden)
		return
	}
	newUUID := uuid.NewString()
	res, err := db.Handle.Exec(`UPDATE frontend SET uuid = ? WHERE uuid = ?`, newUUID, currentUUID)
	if err == nil {
		var n int64
		if n, err = res.RowsAffected(); err == nil && n != 1 {
			err = fmt.Errorf("%d rows affected", n)
		}
	}
	if err != nil {
		http.Error(w, "500 - db update failed", http.StatusInternalServerError)
		log.Println("Error updating UUID:", err)
		return
	}
	be.frontendsMu.Lock() // lookups match on UUID
	f.UUID = newUUID
	be.frontendsMu.Unlock()
	http.Redirect(w, r, path.Join("/sv", f.UUID, f.Name)+"?notice=uuid#settings", http.StatusSeeOther)
}

func WebEditServer(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	f, err := be.FindFrontend(r.PostFormValue("uuid"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	name, port, err := validateNewServer(r.PostFormValue("servername"), r.PostFormValue("serveraddr"), r.PostFormValue("serverport"))
	if err == nil {
		if other, dupe := be.FindFrontendByName(name); dupe == nil && other != f {
			err = fmt.Errorf("a server named %q already exists", name)
		}
	}
	if err != nil {
		renderServerView(w, user, f, nil, err.Error(), "")
		return
	}
	addr := strings.TrimSpace(r.PostFormValue("serveraddr"))
	enabled := r.PostFormValue("switchenabled") == "on"
	teleport := r.PostFormValue("switchteleport") == "on"
	invite := r.PostFormValue("switchinvite") == "on"
	protect := r.PostFormValue("switchprotect") == "on"

	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.Owner != user.Email {
		http.Error(w, "403 - permission denied", http.StatusForbidden)
		return
	}
	// save first so memory doesn't diverge from the database on failure
	qry := "UPDATE frontend SET name=?, ip_address=?, port=?, enabled=?, allow_teleport=?, allow_invite=?, delete_protection=? WHERE id=?"
	if _, err = db.Handle.Exec(qry, name, addr, port, enabled, teleport, invite, protect, f.ID); err != nil {
		http.Error(w, "500 - error updating frontend in database", http.StatusInternalServerError)
		log.Printf("error updating frontend %q: %v\n", f.Name, err)
		return
	}
	f.IPAddress = addr
	f.Port = port
	f.Enabled = enabled
	f.AllowTeleport = teleport
	f.AllowInvite = invite
	f.DeleteProtect = protect
	be.frontendsMu.Lock() // lookups match on Name
	f.Name = name
	be.frontendsMu.Unlock()
	http.Redirect(w, r, path.Join("/sv", f.UUID, name)+"?notice=saved#settings", http.StatusSeeOther)
}

// ServerKeysHandler shows the key page for a frontend, generates a key pair
// to show once (action=Generate), or saves a public key (action=SaveKey).
func ServerKeysHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	fe := frontendFor(w, r, user, true) // keys are settings
	if fe == nil {
		return
	}
	data := PageResponse{SessionUser: user, Frontend: fe}
	data.Head.Title = "Keys | CloudAdmin"
	data.NavHighlight.Servers = "active"

	switch r.PostFormValue("action") {
	case "SaveKey":
		keydata := strings.TrimSpace(r.PostFormValue("keydata"))
		fe.Mu.Lock()
		defer fe.Mu.Unlock()
		back := path.Join("/sv", fe.UUID, fe.Name)
		if _, err := crypto.LoadPublicKey([]byte(keydata)); err != nil {
			http.Redirect(w, r, back+"?notice=badkey#settings", http.StatusSeeOther)
			return
		}
		if _, err := db.Handle.Exec("UPDATE frontend SET public_key_data=? WHERE id=?", keydata, fe.ID); err != nil {
			log.Printf("error saving public key for %q: %v\n", fe.Name, err)
			http.Error(w, "500 - unable to save the key", http.StatusInternalServerError)
			return
		}
		fe.PublicKeyData = keydata
		http.Redirect(w, r, back+"?notice=key#settings", http.StatusSeeOther)
		return
	case "Generate":
		priv, pub, err := crypto.GenerateKeyPair(2048)
		if err != nil {
			log.Println(err)
			http.Error(w, "500 - key generation failed", http.StatusInternalServerError)
			return
		}
		data.KeyPair = &KeyPair{Private: string(priv), Public: string(pub)}
		w.Header().Set("Cache-Control", "no-store") // the private key is only ever shown here
	}

	tmpl, err := parsePage("generate-keys.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, "keygen", data, fe); err != nil {
		log.Println(err)
	}
}

// KeyPair is a freshly generated key pair, PEM-encoded. It's shown once and
// never stored.
type KeyPair struct {
	Private, Public string
}
