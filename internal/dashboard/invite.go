package dashboard

import (
	"net/url"
	"strings"
)

func normalizeTelegramUsername(username string) string {
	return strings.TrimPrefix(strings.TrimSpace(username), "@")
}

func employeeInviteLinks(botUsername, inviteCode string) (string, string) {
	botUsername = normalizeTelegramUsername(botUsername)
	inviteCode = strings.TrimSpace(inviteCode)
	if botUsername == "" || inviteCode == "" {
		return "", ""
	}

	direct := &url.URL{Scheme: "https", Host: "t.me", Path: "/" + botUsername}
	directQuery := direct.Query()
	directQuery.Set("start", inviteCode)
	direct.RawQuery = directQuery.Encode()

	share := &url.URL{Scheme: "https", Host: "t.me", Path: "/share/url"}
	shareQuery := share.Query()
	shareQuery.Set("url", direct.String())
	shareQuery.Set("text", "Mời bạn tham gia lịch làm việc của quán trên ShiftBot.")
	share.RawQuery = shareQuery.Encode()

	return direct.String(), share.String()
}

// ownerTelegramLink builds the deep link owners open to bind their Telegram account.
// Payload format: start=owner_<token>
func ownerTelegramLink(botUsername, token string) string {
	botUsername = normalizeTelegramUsername(botUsername)
	token = strings.TrimSpace(token)
	if botUsername == "" || token == "" {
		return ""
	}
	direct := &url.URL{Scheme: "https", Host: "t.me", Path: "/" + botUsername}
	q := direct.Query()
	q.Set("start", "owner_"+token)
	direct.RawQuery = q.Encode()
	return direct.String()
}
