package jtt809

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestVehicleLocationUploadEncode(t *testing.T) {
	pos := VehiclePosition{
		Encrypt:     0,
		Time:        time.Date(2024, 5, 12, 15, 4, 5, 0, time.UTC),
		Lon:         116397000,
		Lat:         39908000,
		Speed:       60,
		RecordSpeed: 61,
		Mileage:     123456,
		Direction:   180,
		Altitude:    45,
		State:       0x01,
		Alarm:       0x02,
	}
	body := VehicleLocationUpload{
		VehicleNo:    "京A12345",
		VehicleColor: 1,
		Position:     pos,
	}
	data, err := EncodePackage(Package{Header: Header{GNSSCenterID: 99}, Body: body})
	if err != nil {
		t.Fatalf("encode vehicle upload: %v", err)
	}
	frame, err := DecodeFrame(data)
	if err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	if frame.BodyID != MsgIDDynamicInfo {
		t.Fatalf("unexpected body id: %x", frame.BodyID)
	}
	raw := frame.RawBody
	subType := binary.BigEndian.Uint16(raw[22:24])
	if subType != 0x1202 {
		t.Fatalf("unexpected sub type: %x", subType)
	}
	length := binary.BigEndian.Uint32(raw[24:28])
	if length != 36 {
		t.Fatalf("unexpected position length: %d", length)
	}
	position, err := ParseVehiclePosition(raw[28:])
	if err != nil {
		t.Fatalf("parse position: %v", err)
	}
	if position.Direction != pos.Direction || position.Lon != pos.Lon || position.Lat != pos.Lat {
		t.Fatalf("position mismatch: %+v", position)
	}
}

func TestVehicleLocationUploadEncode2019(t *testing.T) {
	gnss := []byte{0x01, 0x02, 0x03, 0x04}
	pos := &VehiclePosition2019{
		Encrypt:     1,
		GnssData:    gnss,
		PlatformID1: "11000000001",
		PlatformID2: "11000000002",
		PlatformID3: "11000000003",
		Alarm1:      1,
		Alarm2:      2,
		Alarm3:      3,
	}
	body := VehicleLocationUpload{
		VehicleNo:    "粤B00001",
		VehicleColor: 2,
		Position2019: pos,
	}
	data, err := EncodePackage(Package{
		Header: Header{GNSSCenterID: 88, WithUTC: true},
		Body:   body,
	})
	if err != nil {
		t.Fatalf("encode 2019 vehicle upload: %v", err)
	}
	frame, err := DecodeFrame(data)
	if err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	if !frame.Header.WithUTC {
		t.Fatalf("expected UTC header for 2019")
	}
	raw := frame.RawBody
	length := binary.BigEndian.Uint32(raw[24:28])
	const expectedLen = 54
	if length != expectedLen {
		t.Fatalf("unexpected 2019 length: %d", length)
	}
	parsed, err := ParseVehiclePosition2019(raw[28:])
	if err != nil {
		t.Fatalf("parse position 2019: %v", err)
	}
	if parsed.Alarm3 != pos.Alarm3 || len(parsed.GnssData) != len(gnss) {
		t.Fatalf("position 2019 mismatch: %+v", parsed)
	}
}

func TestVehicleLocationValidate(t *testing.T) {
	pos := VehiclePosition{
		Time:      time.Now(),
		Lon:       181000000,
		Lat:       0,
		Direction: 10,
	}
	if _, err := pos.encode(); err == nil {
		t.Fatalf("expected lon validation error")
	}
}
