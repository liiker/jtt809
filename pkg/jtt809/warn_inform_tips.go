package jtt809

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
)

// WarnMsgInformTips 表示 0x1403 上报报警预警消息（UP_WARN_MSG_INFORM_TIPS）。
type WarnMsgInformTips struct {
	SourcePlatformID string
	WarnType         WarnType
	WarnTime         time.Time
	StartTime        time.Time
	EndTime          time.Time
	VehicleNo        string
	VehicleColor     PlateColor
	TargetPlatformID string
	DrvLineID        uint32
	WarnLength       uint32
	WarnContent      string
	WarnContentRaw   []byte
}

// ParseWarnMsgInformTips 解析 0x1403 子业务载荷（DATA 字段部分）。
func ParseWarnMsgInformTips(payload []byte) (*WarnMsgInformTips, error) {
	// 表 71 未显式给出车牌长度，这里与 0x1402 保持一致采用 21 字节定长。
	const vehicleNoLen = 21
	const fixedLen = 11 + 2 + 8 + 8 + 8 + vehicleNoLen + 1 + 11 + 4 + 4
	if len(payload) < fixedLen {
		return nil, errors.New("payload too short for warn msg inform tips")
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
	vehicleNo, _ := DecodeGBK(payload[offset : offset+vehicleNoLen])
	offset += vehicleNoLen
	vehicleColor := PlateColor(payload[offset])
	offset++
	targetPlatform := strings.TrimRight(string(payload[offset:offset+11]), "\x00")
	offset += 11
	drvLineID := binary.BigEndian.Uint32(payload[offset : offset+4])
	offset += 4
	warnLen := binary.BigEndian.Uint32(payload[offset : offset+4])
	offset += 4
	if warnLen > uint32(len(payload)-offset) {
		return nil, fmt.Errorf("warn content length mismatch: declare=%d actual=%d", warnLen, len(payload)-offset)
	}
	if warnLen > 1024 {
		return nil, fmt.Errorf("warn content length exceeds 1024: %d", warnLen)
	}
	end := offset + int(warnLen)
	raw := make([]byte, warnLen)
	copy(raw, payload[offset:end])
	content, _ := DecodeGBK(raw)

	return &WarnMsgInformTips{
		SourcePlatformID: srcPlatform,
		WarnType:         warnType,
		WarnTime:         warnTime,
		StartTime:        startTime,
		EndTime:          endTime,
		VehicleNo:        vehicleNo,
		VehicleColor:     vehicleColor,
		TargetPlatformID: targetPlatform,
		DrvLineID:        drvLineID,
		WarnLength:       warnLen,
		WarnContent:      content,
		WarnContentRaw:   raw,
	}, nil
}

// ParseWarnMsgInformPacket 校验并解析 0x1400 主业务下的 0x1403 子业务。
func ParseWarnMsgInformPacket(body []byte) (*WarnMsgInformTips, error) {
	pkt, err := ParseAlarmInfo(body)
	if err != nil {
		return nil, err
	}
	if pkt.SubBusinessID != UP_WARN_MSG_INFORM_TIPS {
		return nil, fmt.Errorf("unexpected sub business id: %x", pkt.SubBusinessID)
	}
	return ParseWarnMsgInformTips(pkt.Payload)
}
