package jtt809

import (
	"encoding/hex"
	"testing"
)

func TestTimeTokenReportEncode(t *testing.T) {
	code1 := make([]byte, 64)
	code2 := make([]byte, 64)
	for i := 0; i < 64; i++ {
		code1[i] = byte(i + 1)
		code2[i] = byte(0xA0 + i)
	}
	body := TimeTokenReport{
		VehicleNo:    "粤A12345",
		VehicleColor: VehicleColorBlue,
		PlatformID:   "PLATID00001",
		AuthCode1:    code1,
		AuthCode2:    code2,
	}
	data, err := EncodePackage(Package{Header: Header{GNSSCenterID: 0x01020304}, Body: body})
	if err != nil {
		t.Fatalf("encode time token report: %v", err)
	}
	frame, err := DecodeFrame(data)
	if err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	if frame.BodyID != 0x1700 {
		t.Fatalf("unexpected body id: %x", frame.BodyID)
	}
	sub, err := ParseSubBusiness(frame.RawBody)
	if err != nil {
		t.Fatalf("parse sub business: %v", err)
	}
	if sub.SubBusinessID != SubMsgTimeTokenReport {
		t.Fatalf("unexpected sub business id: %x", sub.SubBusinessID)
	}
	if sub.PayloadLength != 139 {
		t.Fatalf("unexpected payload length: %d", sub.PayloadLength)
	}
	parsed, err := ParseTimeTokenReport(sub.Payload)
	if err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	if parsed.PlatformID != body.PlatformID {
		t.Fatalf("platform id mismatch: %s", parsed.PlatformID)
	}
	if len(parsed.AuthCode1) != 64 || len(parsed.AuthCode2) != 64 {
		t.Fatalf("auth code length mismatch: %d/%d", len(parsed.AuthCode1), len(parsed.AuthCode2))
	}
	if parsed.AuthCode1[0] != 1 || parsed.AuthCode2[0] != 0xA0 {
		t.Fatalf("auth code content mismatch")
	}
}

func TestTimeTokenReportParseDemoHex(t *testing.T) {
	hexStr := "5B000000C9000006821700013415F4010000000000270F000000005E02A507B8D4C1413132333435000000000000000000000000000217010000008B01020304050607080910110000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000E7D35D"
	data, _ := hex.DecodeString(hexStr)
	frame, err := DecodeFrame(data)
	if err != nil {
		t.Fatalf("decode demo frame: %v", err)
	}
	if frame.BodyID != 0x1700 {
		t.Fatalf("unexpected body id: %x", frame.BodyID)
	}
	sub, err := ParseSubBusiness(frame.RawBody)
	if err != nil {
		t.Fatalf("parse sub business: %v", err)
	}
	if sub.SubBusinessID != SubMsgTimeTokenReport {
		t.Fatalf("unexpected sub business id: %x", sub.SubBusinessID)
	}
	if sub.PayloadLength != 139 {
		t.Fatalf("unexpected payload length: %d", sub.PayloadLength)
	}
	payload, err := ParseTimeTokenReport(sub.Payload)
	if err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	if len(payload.AuthCode1) != 64 || len(payload.AuthCode2) != 64 {
		t.Fatalf("unexpected auth code length: %d/%d", len(payload.AuthCode1), len(payload.AuthCode2))
	}
}
