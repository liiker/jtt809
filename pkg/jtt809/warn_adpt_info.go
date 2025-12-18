package jtt809

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
)

// AlarmInfoPacket 表示 0x1400 报警信息交互业务的通用封装，仅包含子业务标识与载荷。
type AlarmInfoPacket struct {
	SubBusinessID uint16
	PayloadLength uint32
	Payload       []byte
}

// ParseAlarmInfo 解析 0x1400 主业务体，获取子业务标识与后续载荷。
func ParseAlarmInfo(body []byte) (*AlarmInfoPacket, error) {
	if len(body) < 6 {
		return nil, errors.New("alarm info body too short")
	}
	sub := binary.BigEndian.Uint16(body[:2])
	length := binary.BigEndian.Uint32(body[2:6])
	if int(length) != len(body)-6 {
		return nil, fmt.Errorf("payload length mismatch: declare=%d actual=%d", length, len(body)-6)
	}
	payload := make([]byte, len(body[6:]))
	copy(payload, body[6:])
	return &AlarmInfoPacket{
		SubBusinessID: sub,
		PayloadLength: length,
		Payload:       payload,
	}, nil
}

// WarnMsgAdptInfo 表示 0x1402 上报报警信息消息的数据体。
type WarnMsgAdptInfo struct {
	SourcePlatformID string
	WarnType         WarnType
	WarnTime         time.Time
	StartTime        time.Time
	EndTime          time.Time
	VehicleNo        string
	VehicleColor     PlateColor
	TargetPlatformID string
	DrvLineID        uint32
	InfoLength       uint32
	InfoContent      string
	InfoContentRaw   []byte
}

// ParseWarnMsgAdptInfo 解析 0x1402 子业务载荷（DATA 字段部分）。
func ParseWarnMsgAdptInfo(payload []byte) (*WarnMsgAdptInfo, error) {
	const fixedLen = 11 + 2 + 8 + 8 + 8 + 21 + 1 + 11 + 4 + 4
	if len(payload) < fixedLen {
		return nil, errors.New("payload too short for warn msg adpt info")
	}
	offset := 0
	srcPlatform := strings.TrimRight(string(payload[offset:offset+11]), "\x00")
	offset += 11
	warnType := WarnType(binary.BigEndian.Uint16(payload[offset : offset+2]))
	offset += 2
	warnTime := parseUTCSeconds(payload[offset : offset+8])
	offset += 8
	startTime := parseUTCSeconds(payload[offset : offset+8])
	offset += 8
	endTime := parseUTCSeconds(payload[offset : offset+8])
	offset += 8
	vehicleNo, _ := DecodeGBK(payload[offset : offset+21])
	offset += 21
	vehicleColor := PlateColor(payload[offset])
	offset++
	targetPlatform := strings.TrimRight(string(payload[offset:offset+11]), "\x00")
	offset += 11
	drvLineID := binary.BigEndian.Uint32(payload[offset : offset+4])
	offset += 4
	infoLen := binary.BigEndian.Uint32(payload[offset : offset+4])
	offset += 4
	if infoLen > uint32(len(payload)-offset) {
		return nil, fmt.Errorf("info length mismatch: declare=%d actual=%d", infoLen, len(payload)-offset)
	}
	if infoLen > 1024 {
		return nil, fmt.Errorf("info length exceeds 1024: %d", infoLen)
	}
	end := offset + int(infoLen)
	infoRaw := make([]byte, infoLen)
	copy(infoRaw, payload[offset:end])
	infoText, _ := DecodeGBK(infoRaw)

	return &WarnMsgAdptInfo{
		SourcePlatformID: srcPlatform,
		WarnType:         warnType,
		WarnTime:         warnTime,
		StartTime:        startTime,
		EndTime:          endTime,
		VehicleNo:        vehicleNo,
		VehicleColor:     vehicleColor,
		TargetPlatformID: targetPlatform,
		DrvLineID:        drvLineID,
		InfoLength:       infoLen,
		InfoContent:      infoText,
		InfoContentRaw:   infoRaw,
	}, nil
}

// ParseWarnMsgAdptPacket 校验并解析 0x1400 主业务下的 0x1402 子业务。
func ParseWarnMsgAdptPacket(body []byte) (*WarnMsgAdptInfo, error) {
	pkt, err := ParseAlarmInfo(body)
	if err != nil {
		return nil, err
	}
	if pkt.SubBusinessID != UP_WARN_MSG_ADPT_INFO {
		return nil, fmt.Errorf("unexpected sub business id: %x", pkt.SubBusinessID)
	}
	return ParseWarnMsgAdptInfo(pkt.Payload)
}
