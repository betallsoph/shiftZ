package telegram

import "testing"

func TestIsGroupChat(t *testing.T) {
	if !isGroupChat(Chat{Type: "group"}) {
		t.Fatal("group")
	}
	if !isGroupChat(Chat{Type: "supergroup"}) {
		t.Fatal("supergroup")
	}
	if isGroupChat(Chat{Type: "private"}) {
		t.Fatal("private should not be group")
	}
}

func TestIsPrivateChat(t *testing.T) {
	if !isPrivateChat(Chat{Type: "private"}) {
		t.Fatal("private")
	}
	if isPrivateChat(Chat{Type: "group"}) {
		t.Fatal("group should not be private")
	}
}
