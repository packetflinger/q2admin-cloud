// Website pages anyone can see without signing in: the home page, sign-in,
// privacy policy and terms of use.

package backend

import (
	"github.com/packetflinger/q2admind/api/website"
	"html/template"
	"log"
	"net/http"
	"strings"
)

// providerName turns an OAuth type like "GOOGLE" into "Google".
func providerName(t string) string {
	if t == "" {
		return ""
	}
	return t[:1] + strings.ToLower(t[1:])
}

// the "index" handler
func WebsiteHandlerIndex(w http.ResponseWriter, r *http.Request) {
	_, e := GetSessionUser(r)
	if e != nil {
		RedirectToSignon(w, r)
		return
	}

	http.Redirect(w, r, Routes.Dashboard, http.StatusSeeOther) // 303
}

// Display signin page
func WebsiteHandlerSignin(w http.ResponseWriter, r *http.Request) {
	if AuthBypass != "" {
		MockLogin(w, r, AuthBypass)
		return
	}
	tmpl, e := template.ParseFS(website.Files, "templates/new/sign-in.tmpl")

	var page PageResponse
	for i := range Website.Creds {
		page.AuthProviders = append(page.AuthProviders, AuthProvider{
			URL:     BuildAuthURL(Website.Creds[i], i),
			Icon:    Website.Creds[i].GetImagePath(),
			Alt:     Website.Creds[i].GetAlternateText(),
			Name:    providerName(Website.Creds[i].GetType().String()),
			Enabled: !Website.Creds[i].GetDisabled(),
		})
	}
	if e != nil {
		log.Println(e)
	} else {
		tmpl.Execute(w, page)
	}
}

func PrivacyHandler(w http.ResponseWriter, r *http.Request) {
	renderInfoPage(w, r, "privacy-policy.tmpl", "privacy-policy", "Privacy Policy | CloudAdmin")
}

func TermsHandler(w http.ResponseWriter, r *http.Request) {
	renderInfoPage(w, r, "terms-of-use.tmpl", "terms-of-use", "Terms of Use | CloudAdmin")
}

// renderInfoPage shows a page that doesn't need a signed-in user, like the
// privacy policy, which the sign-in page links to.
func renderInfoPage(w http.ResponseWriter, r *http.Request, file, name, title string) {
	user, _ := GetSessionUser(r) // nil when signed out
	data := PageResponse{SessionUser: user}
	data.Head.Title = title
	tmpl, err := parsePage(file)
	if err != nil {
		log.Println(err)
		return
	}
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Println(err)
	}
}
