package jtt809

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// PlatformQueryAck 对应子业务 0x1301 平台查岗应答，描述查岗对象与应答内容。
type PlatformQueryAck struct {
	ObjectType     byte
	Responder      string
	ResponderTel   string
	ObjectID       string
	SourceDataType uint16
	SourceMsgSN    uint32
	InfoContent    string
}

// ParsePlatformQueryAck 解析平台查岗应答载荷，返回结构化结果。
func ParsePlatformQueryAck(pkt *SubBusinessPacket) (*PlatformQueryAck, error) {
	if pkt == nil {
		return nil, errors.New("nil packet")
	}
	if pkt.SubBusinessID != SubMsgPlatformQueryAck {
		return nil, fmt.Errorf("unsupported sub business id: %x", pkt.SubBusinessID)
	}
	p := pkt.Payload
	if len(p) < 1+16+20+20+2+4+4 {
		return nil, errors.New("payload too short for platform query ack")
	}
	offset := 0
	objType := p[offset]
	offset++
	responder, _ := DecodeGBK(p[offset : offset+16])
	offset += 16
	tel, _ := DecodeGBK(p[offset : offset+20])
	offset += 20
	objectID, _ := DecodeGBK(p[offset : offset+20])
	offset += 20
	srcType := binary.BigEndian.Uint16(p[offset : offset+2])
	offset += 2
	srcSN := binary.BigEndian.Uint32(p[offset : offset+4])
	offset += 4
	infoLen := binary.BigEndian.Uint32(p[offset : offset+4])
	offset += 4
	if int(infoLen) > len(p)-offset {
		return nil, errors.New("info length exceeds payload")
	}
	info, _ := DecodeGBK(p[offset : offset+int(infoLen)])
	return &PlatformQueryAck{
		ObjectType:     objType,
		Responder:      responder,
		ResponderTel:   tel,
		ObjectID:       objectID,
		SourceDataType: srcType,
		SourceMsgSN:    srcSN,
		InfoContent:    info,
	}, nil
}
