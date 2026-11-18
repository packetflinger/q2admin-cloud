package backend

import (
	"bytes"
	"github.com/packetflinger/q2admind/api/website"
	"html/template"
	"strings"
	"testing"
)

func TestInfoPagesRenderSignedOut(t *testing.T) {
	for file, name := range map[string]string{"privacy-policy.tmpl": "privacy-policy", "terms-of-use.tmpl": "terms-of-use"} {
		if out := renderPage(t, file, name, PageResponse{}); strings.Contains(out, "Account menu") {
			t.Errorf("%s shows an account menu with nobody signed in", name)
		}
	}
}

func TestSignInPage(t *testing.T) {
	tmpl := template.Must(template.ParseFS(website.Files, "templates/new/sign-in.tmpl"))
	var out bytes.Buffer
	data := PageResponse{AuthProviders: []AuthProvider{{URL: "/auth/google", Icon: "/g.svg", Name: providerName("GOOGLE"), Enabled: true}, {Name: "Off"}}}
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `you agree to the <a href="/terms-of-use">Terms</a> and <a href="/privacy-policy">Privacy policy</a>`) {
		t.Error("sign-in page is missing the terms and privacy agreement line")
	}
	if !strings.Contains(out.String(), "Continue with Google") || strings.Contains(out.String(), "Off") {
		t.Error("sign-in page should list only enabled providers")
	}
}
