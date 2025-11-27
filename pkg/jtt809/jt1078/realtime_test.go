package jt1078

import (
	"testing"
)

func TestDownRealTimeVideoStartupReq(t *testing.T) {
	req := DownRealTimeVideoStartupReq{
		ChannelID:     1,
		AVItemType:    2,
		AuthorizeCode: "AUTH_CODE_REQ",
		GnssData:      make([]byte, 36),
	}
	// Fill GNSS data with some dummy values
	for i := range req.GnssData {
		req.GnssData[i] = byte(i)
	}

	encoded, err := req.Encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// 1 + 1 + 64 + 36 = 102
	if len(encoded) != 102 {
		t.Fatalf("unexpected length: %d", len(encoded))
	}

	decoded, err := ParseDownRealTimeVideoStartupReq(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.ChannelID != req.ChannelID {
		t.Errorf("expected channel id %d, got %d", req.ChannelID, decoded.ChannelID)
	}
	if decoded.AVItemType != req.AVItemType {
		t.Errorf("expected av item type %d, got %d", req.AVItemType, decoded.AVItemType)
	}
	if decoded.AuthorizeCode != req.AuthorizeCode {
		t.Errorf("expected auth code %s, got %s", req.AuthorizeCode, decoded.AuthorizeCode)
	}
	if len(decoded.GnssData) != 36 {
		t.Fatalf("expected gnss data len 36, got %d", len(decoded.GnssData))
	}
	if string(decoded.GnssData) != string(req.GnssData) {
		t.Errorf("gnss data mismatch")
	}
}

func TestRealTimeVideoStartupAck(t *testing.T) {
	ack := RealTimeVideoStartupAck{
		Result:     0,
		ServerIP:   "192.168.1.100",
		ServerPort: 8080,
	}

	encoded, err := ack.Encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// 1 + 32 + 2 = 35
	if len(encoded) != 35 {
		t.Fatalf("unexpected length: %d", len(encoded))
	}

	decoded, err := ParseRealTimeVideoStartupAck(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.Result != ack.Result {
		t.Errorf("expected result %d, got %d", ack.Result, decoded.Result)
	}
	if decoded.ServerIP != ack.ServerIP {
		t.Errorf("expected server ip %s, got %s", ack.ServerIP, decoded.ServerIP)
	}
	if decoded.ServerPort != ack.ServerPort {
		t.Errorf("expected server port %d, got %d", ack.ServerPort, decoded.ServerPort)
	}
}
