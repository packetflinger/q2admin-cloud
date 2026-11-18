// Website user names and the profile page.

package backend

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	pb "github.com/packetflinger/q2admind/proto"
)

// DisplayName is how a user is shown to others, such as in console echoes:
// the name they set on their profile, or the part of their email before the @.
func DisplayName(u *pb.User) string {
	if u.GetName() != "" {
		return u.GetName()
	}
	local, _, _ := strings.Cut(u.GetEmail(), "@")
	return local
}

// displayNameFor is DisplayName for the user with this email.
func displayNameFor(email string) string {
	u, _ := be.GetUserByEmail(email)
	if u == nil {
		u = &pb.User{Email: email}
	}
	return DisplayName(u)
}

// SetUserName saves the name u is shown as; empty clears it. Names go into
// console lines and logs, so they're one line of up to 32 characters.
// shortcut: users aren't locked, so a page rendering u while it's renamed
// could race; add a users lock if renames become frequent.
func SetUserName(u *pb.User, name string) error {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > 32 || strings.ContainsFunc(name, unicode.IsControl) {
		return errors.New("names are up to 32 characters on one line")
	}
	if _, err := db.Handle.Exec("UPDATE user SET name = ? WHERE email = ?", name, u.GetEmail()); err != nil {
		log.Printf("error saving %q's name: %v\n", u.GetEmail(), err)
		return errors.New("unable to save your name, try again")
	}
	u.Name = name
	return nil
}

// ProfileHandler shows the signed-in user's account details (GET), or sets
// their name (POST).
func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	user, err := GetSessionUser(r)
	if err != nil {
		RedirectToSignon(w, r)
		return
	}
	data := PageResponse{SessionUser: user, Notice: notices[r.URL.Query().Get("notice")]}
	if r.Method == http.MethodPost {
		if err := SetUserName(user, r.PostFormValue("name")); err != nil {
			data.FormError, data.Form = err.Error(), r.PostForm
		} else {
			http.Redirect(w, r, "/me?notice=namesaved", http.StatusSeeOther)
			return
		}
	}
	data.Frontends = FrontendsByIdentity(user.GetEmail())
	data.Head.Title = "Profile | CloudAdmin"
	tmpl, err := parsePage("profile.tmpl")
	if err != nil {
		log.Println(err)
		return
	}
	if err := executeLocked(w, tmpl, "profile", data, data.Frontends...); err != nil {
		log.Println(err)
	}
}
