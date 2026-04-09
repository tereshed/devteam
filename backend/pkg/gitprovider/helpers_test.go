package gitprovider

import (
	"net/url"
	"strings"
	"testing"
)

func TestSanitizeToken_userinfoEncodingMatchesURLPackage(t *testing.T) {
	t.Parallel()
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

func TestSanitizeToken_emptyTokenNoOp(t *testing.T) {
	t.Parallel()
	if got := sanitizeToken("keep-me", ""); got != "keep-me" {
		t.Fatal(got)
	}
}

func TestIsGitBlobOrPathMissing(t *testing.T) {
	t.Parallel()
	if !isGitBlobOrPathMissing("fatal: path README does not exist") {
		t.Fatal("expected true")
	}
	if isGitBlobOrPathMissing("ok") {
		t.Fatal("expected false")
	}
}

func TestUserinfoEncodedPassword_spaceUsesPercent20NotPlus(t *testing.T) {
	t.Parallel()
	enc := userinfoEncodedPassword("a b")
	if enc == "a+b" || strings.Contains(enc, "+") {
		t.Fatalf("got %q; want %%20 for space like net/url userinfo, not QueryEscape +", enc)
	}
	if enc != "a%20b" {
		t.Fatalf("got %q, want a%%20b", enc)
	}
}

func TestMapGitCLIError_table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		stderr string
		want   error
	}{
		{"Authentication failed", ErrAuthFailed},
		{"could not read Username", ErrAuthFailed},
		{"Access denied", ErrAuthFailed},
		{"Invalid username or password", ErrAuthFailed},
		{"repository not found", ErrRepoNotFound},
		{"random network glitch", ErrRepoNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.stderr, func(t *testing.T) {
			t.Parallel()
			got := mapGitCLIError(tc.stderr)
			if got != tc.want {
				t.Fatalf("mapGitCLIError(%q) = %v, want %v", tc.stderr, got, tc.want)
			}
		})
	}
}

func TestSanitizeToken_QueryEscape_plusForm(t *testing.T) {
	t.Parallel()
	tok := "ab+cd"
	s := "https://x/?q=" + tok + "&z=1"
	out := sanitizeToken(s, tok)
	if strings.Contains(out, tok) {
		t.Fatalf("raw + form leaked: %q", out)
	}
}
