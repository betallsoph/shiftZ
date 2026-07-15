package dashboard

import (
	"net/url"
	"testing"
)

func TestEmployeeInviteLinks(t *testing.T) {
	direct, share := employeeInviteLinks("@shiftzz_bot", "invite 01")
	if direct != "https://t.me/shiftzz_bot?start=invite+01" {
		t.Fatalf("direct = %q", direct)
	}

	parsed, err := url.Parse(share)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != "t.me" || parsed.Path != "/share/url" {
		t.Fatalf("share = %q", share)
	}
	if got := parsed.Query().Get("url"); got != direct {
		t.Fatalf("shared URL = %q, want %q", got, direct)
	}
}

func TestEmployeeInviteLinksRequireUsernameAndCode(t *testing.T) {
	for _, tc := range []struct {
		username string
		code     string
	}{
		{username: "", code: "invite01"},
		{username: "shiftzz_bot", code: ""},
	} {
		direct, share := employeeInviteLinks(tc.username, tc.code)
		if direct != "" || share != "" {
			t.Fatalf("employeeInviteLinks(%q, %q) = %q, %q", tc.username, tc.code, direct, share)
		}
	}
}
