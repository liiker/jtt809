package jtt809

import "testing"

func TestHeartbeatEncode(t *testing.T) {
	data, err := EncodePackage(Package{
		Header: Header{GNSSCenterID: 77},
		Body:   HeartbeatRequest{},
	})
	if err != nil {
		t.Fatalf("encode heartbeat: %v", err)
	}
	frame, err := DecodeFrame(data)
	if err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	if frame.BodyID != UP_LINKTEST_REQ {
		t.Fatalf("unexpected body id: %x", frame.BodyID)
	}
	if len(frame.RawBody) != 0 {
		t.Fatalf("heartbeat body should be empty")
	}
}
