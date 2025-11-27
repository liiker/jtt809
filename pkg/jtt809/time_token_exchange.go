package jtt809

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

// TimeTokenReport 表示主链路时效口令业务（0x1700）中的 0x1701 子业务载荷。
type TimeTokenReport struct {
	VehicleNo    string
	VehicleColor byte

	PlatformID string // 11 字节
	AuthCode1  []byte // 64 字节
	AuthCode2  []byte // 64 字节
}

func (TimeTokenReport) MsgID() uint16 { return 0x1700 }

// Encode 构造 0x1700 主业务封装的 0x1701 子业务报文。
func (t TimeTokenReport) Encode() ([]byte, error) {
	if len(t.VehicleNo) == 0 {
		return nil, errors.New("vehicle number is required")
	}
	if len(t.PlatformID) == 0 {
		return nil, errors.New("platform id is required")
	}
	if len(t.AuthCode1) != 64 || len(t.AuthCode2) != 64 {
		return nil, fmt.Errorf("auth codes must be 64 bytes each, got %d/%d", len(t.AuthCode1), len(t.AuthCode2))
	}
	payload, err := t.encodePayload()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.Write(PadRightGBK(t.VehicleNo, 21))
	buf.WriteByte(t.VehicleColor)
	_ = binary.Write(&buf, binary.BigEndian, SubMsgTimeTokenReport)
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(payload)))
	buf.Write(payload)
	return buf.Bytes(), nil
}

func (t TimeTokenReport) encodePayload() ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(PadRightGBK(t.PlatformID, 11))
	buf.Write(t.AuthCode1)
	buf.Write(t.AuthCode2)
	return buf.Bytes(), nil
}

// ParseTimeTokenReport 解析 0x1701 子业务载荷。
func ParseTimeTokenReport(payload []byte) (*TimeTokenReport, error) {
	if len(payload) < 11+64+64 {
		return nil, errors.New("payload too short for time token report")
	}
	platform := padTrim(payload[:11])
	code1 := append([]byte(nil), payload[11:11+64]...)
	code2 := append([]byte(nil), payload[11+64:11+64+64]...)
	return &TimeTokenReport{
		PlatformID: platform,
		AuthCode1:  code1,
		AuthCode2:  code2,
	}, nil
}
