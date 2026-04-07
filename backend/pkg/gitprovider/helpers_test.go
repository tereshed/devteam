package gitprovider

import (
	"net/url"
	"strings"
	"testing"
)

func TestSanitizeToken_userinfoEncodingMatchesURLPackage(t *testing.T) {
	tok := "ghp_secret with space"
	u, err := url.Parse("https://github.com/o/r.git")
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(gitHTTPAccessTokenUser, tok)
	withTok := u.String()
	if strings.Contains(withTok, tok) && strings.Contains(withTok, " ") {
		t.Fatal("expected token to be percent-encoded in URL string")
	}
	san := sanitizeToken(withTok, tok)
	if strings.Contains(san, tok) {
		t.Fatalf("raw token leaked: %q", san)
	}
	enc := userinfoEncodedPassword(tok)
	if enc != "" && strings.Contains(san, enc) {
		t.Fatalf("encoded token leaked: %q", san)
	}
}

func TestUserinfoEncodedPassword_spaceUsesPercent20NotPlus(t *testing.T) {
	enc := userinfoEncodedPassword("a b")
	if enc == "a+b" || strings.Contains(enc, "+") {
		t.Fatalf("got %q; want %%20 for space like net/url userinfo, not QueryEscape +", enc)
	}
	if enc != "a%20b" {
		t.Fatalf("got %q, want a%%20b", enc)
	}
}
