package jt1078

import (
	"testing"
)

func TestAuthorizeStartupReq(t *testing.T) {
	req := AuthorizeStartupReq{
		PlatformID:     "PLAT123",
		AuthorizeCode1: "AUTH_CODE_1",
		AuthorizeCode2: "AUTH_CODE_2",
	}

	encoded, err := req.Encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	if len(encoded) != 11+64+64 {
		t.Fatalf("unexpected length: %d", len(encoded))
	}

	decoded, err := ParseAuthorizeStartupReq(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.PlatformID != req.PlatformID {
		t.Errorf("expected platform id %s, got %s", req.PlatformID, decoded.PlatformID)
	}
	if decoded.AuthorizeCode1 != req.AuthorizeCode1 {
		t.Errorf("expected auth code 1 %s, got %s", req.AuthorizeCode1, decoded.AuthorizeCode1)
	}
	if decoded.AuthorizeCode2 != req.AuthorizeCode2 {
		t.Errorf("expected auth code 2 %s, got %s", req.AuthorizeCode2, decoded.AuthorizeCode2)
	}
}

func TestAuthorizeStartupReq_ShortBody(t *testing.T) {
	_, err := ParseAuthorizeStartupReq([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for short body, got nil")
	}
}
