package jtt809

import "testing"

func TestLogoutEncodeDecode(t *testing.T) {
	header := Header{GNSSCenterID: 1}
	req := LogoutRequest{UserID: 100, Password: "pwd123"}
	data, err := EncodePackage(Package{Header: header, Body: req})
	if err != nil {
		t.Fatalf("encode logout request: %v", err)
	}
	frame, err := DecodeFrame(data)
	if err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	if frame.BodyID != MsgIDLogoutRequest {
		t.Fatalf("unexpected body id: %x", frame.BodyID)
	}
}

func TestLogoutAutoResponse(t *testing.T) {
	req := LogoutRequest{UserID: 1, Password: "pwd"}
	data, _ := EncodePackage(Package{Header: Header{GNSSCenterID: 2}, Body: req})
	frame, err := DecodeFrame(data)
	if err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	respPkg, err := GenerateResponse(frame, nil)
	if err != nil {
		t.Fatalf("generate response: %v", err)
	}
	if respPkg.Header.BusinessType != MsgIDLogoutResponse {
		t.Fatalf("unexpected resp id: %x", respPkg.Header.BusinessType)
	}
}

func TestParseDisconnectInform(t *testing.T) {
	data, err := EncodePackage(Package{
		Header: Header{GNSSCenterID: 10},
		Body:   DisconnectInform{ErrorCode: 2},
	})
	if err != nil {
		t.Fatalf("encode disconnect inform: %v", err)
	}
	frame, err := DecodeFrame(data)
	if err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	disc, err := ParseDisconnectInform(frame)
	if err != nil {
		t.Fatalf("parse disconnect: %v", err)
	}
	if disc.ErrorCode != 2 {
		t.Fatalf("unexpected error code: %d", disc.ErrorCode)
	}
}
