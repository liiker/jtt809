package jtt809

import (
	"testing"
	"time"
)

func TestParseBatchLocation(t *testing.T) {
	pos1 := VehiclePosition{Encrypt: 0, Time: time.Date(2024, 1, 24, 10, 11, 12, 0, time.UTC), Lon: 258, Lat: 772, Speed: 50, RecordSpeed: 51, Mileage: 1, Direction: 90, Altitude: 5, State: 1, Alarm: 2}
	pos2 := VehiclePosition{Encrypt: 0, Time: time.Date(2024, 1, 25, 13, 14, 15, 0, time.UTC), Lon: 1286, Lat: 1800, Speed: 60, RecordSpeed: 61, Mileage: 3, Direction: 100, Altitude: 6, State: 3, Alarm: 4}
	b1, _ := pos1.encode()
	b2, _ := pos2.encode()
	locPayload := append([]byte{2}, append(b1, b2...)...)

	pkt := &SubBusinessPacket{
		Plate:         "TEST",
		Color:         VehicleColorYellow,
		SubBusinessID: SubMsgBatchLocation,
		Payload:       locPayload,
		PayloadLength: uint32(len(locPayload)),
	}
	result, err := ParseBatchLocation(pkt)
	if err != nil {
		t.Fatalf("parse batch location: %v", err)
	}
	if len(result.Locations) != 2 {
		t.Fatalf("expect 2 locations, got %d", len(result.Locations))
	}
	if result.Locations[0].Speed != 50 || result.Locations[1].Speed != 60 {
		t.Fatalf("unexpected speeds: %d %d", result.Locations[0].Speed, result.Locations[1].Speed)
	}
}

func TestParsePlatformQueryAck(t *testing.T) {
	payload := make([]byte, 1+16+20+20+2+4+4+6)
	offset := 0
	payload[offset] = 1
	offset++
	copy(payload[offset:], []byte("responder\x00\x00\x00\x00\x00\x00"))
	offset += 16
	copy(payload[offset:], []byte("13800138000\x00\x00\x00\x00\x00\x00\x00\x00\x00"))
	offset += 20
	copy(payload[offset:], []byte("OBJID123456789012"))
	offset += 20
	payload[offset] = 0x13
	payload[offset+1] = 0x01
	offset += 2
	payload[offset+0] = 0
	payload[offset+1] = 0
	payload[offset+2] = 0
	payload[offset+3] = 5
	offset += 4
	payload[offset+0] = 0
	payload[offset+1] = 0
	payload[offset+2] = 0
	payload[offset+3] = 6
	offset += 4
	copy(payload[offset:], []byte("infos!"))

	pkt := &SubBusinessPacket{
		Plate:         "TEST",
		Color:         VehicleColorBlue,
		SubBusinessID: SubMsgPlatformQueryAck,
		Payload:       payload,
		PayloadLength: uint32(len(payload)),
	}
	info, err := ParsePlatformQueryAck(pkt)
	if err != nil {
		t.Fatalf("parse platform ack: %v", err)
	}
	if info.InfoContent != "infos!" {
		t.Fatalf("unexpected info content: %s", info.InfoContent)
	}
	if info.SourceDataType != 0x1301 || info.SourceMsgSN != 5 {
		t.Fatalf("unexpected source info: %x %d", info.SourceDataType, info.SourceMsgSN)
	}
}
