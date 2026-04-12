package smbfs

import "testing"

func TestServerShareLookupIsCaseInsensitive(t *testing.T) {
	server, err := NewServer(ServerOptions{})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	if err := server.AddShare(nil, ShareOptions{ShareName: "share", SharePath: "/"}); err != nil {
		t.Fatalf("AddShare() error = %v", err)
	}

	if got := server.GetShare("share"); got == nil {
		t.Fatal("GetShare(lowercase) = nil, want share")
	}
	if got := server.GetShare("SHARE"); got == nil {
		t.Fatal("GetShare(uppercase) = nil, want share")
	}
	if got := server.GetShare("ShArE"); got == nil {
		t.Fatal("GetShare(mixed case) = nil, want share")
	}
}
