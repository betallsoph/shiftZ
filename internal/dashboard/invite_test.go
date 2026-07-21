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

func TestOwnerTelegramLink(t *testing.T) {
	got := ownerTelegramLink("@shiftzz_bot", "abc123")
	if got != "https://t.me/shiftzz_bot?start=owner_abc123" {
		t.Fatalf("got %q", got)
	}
	if ownerTelegramLink("", "abc") != "" || ownerTelegramLink("bot", "") != "" {
		t.Fatal("expected empty link when username or token missing")
	}
}
