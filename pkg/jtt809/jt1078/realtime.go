package jt1078

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"

	"github.com/zboyco/jtt809/pkg/jtt809"
)

// DownRealTimeVideoStartupReq 对应 DOWN_REAL_VIDEO_MSG_STARTUP_REQ (0x9801)
// 实时音视频请求
type DownRealTimeVideoStartupReq struct {
	ChannelID     byte
	AVItemType    byte
	AuthorizeCode string // 64 bytes
	GnssData      []byte // 36 bytes, optional
}

func (DownRealTimeVideoStartupReq) MsgID() uint16 { return jtt809.DOWN_REALVIDEO_MSG_STARTUP }

func (r DownRealTimeVideoStartupReq) Encode() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(r.ChannelID)
	buf.WriteByte(r.AVItemType)
	buf.Write(jtt809.PadRightGBK(r.AuthorizeCode, 64))
	if len(r.GnssData) > 0 {
		if len(r.GnssData) != 36 {
			return nil, errors.New("gnss data must be 36 bytes")
		}
		buf.Write(r.GnssData)
	}
	return buf.Bytes(), nil
}

func ParseDownRealTimeVideoStartupReq(body []byte) (DownRealTimeVideoStartupReq, error) {
	if len(body) < 1+1+64 {
		return DownRealTimeVideoStartupReq{}, errors.New("realtime video request body too short")
	}
	ac, _ := jtt809.DecodeGBK(body[2:66])
	req := DownRealTimeVideoStartupReq{
		ChannelID:     body[0],
		AVItemType:    body[1],
		AuthorizeCode: ac,
	}
	if len(body) >= 66+36 {
		req.GnssData = make([]byte, 36)
		copy(req.GnssData, body[66:66+36])
	}
	return req, nil
}

// RealTimeVideoStartupAck 对应 UP_REAL_VIDEO_MSG_STARTUP_ACK (0x1801)
// 实时音视频请求应答
type RealTimeVideoStartupAck struct {
	Result     byte
	ServerIP   string // 32 bytes
	ServerPort uint16
}

func (RealTimeVideoStartupAck) MsgID() uint16 { return jtt809.UP_REALVIDEO_MSG_STARTUP_ACK }

func (r RealTimeVideoStartupAck) Encode() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(r.Result)
	// Note: C# implementation uses PadLeft for ServerIP in 0x1801
	buf.Write(jtt809.PadLeftGBK(r.ServerIP, 32))
	_ = binary.Write(&buf, binary.BigEndian, r.ServerPort)
	return buf.Bytes(), nil
}

func ParseRealTimeVideoStartupAck(body []byte) (RealTimeVideoStartupAck, error) {
	if len(body) < 1+32+2 {
		return RealTimeVideoStartupAck{}, errors.New("realtime video ack body too short")
	}
	// DecodeGBK handles trimming \x00 from right, but for PadLeft we might have \x00 on left.
	// However, standard string trimming usually handles both or we rely on DecodeGBK to just decode what's there.
	// Actually, GBK decoder might treat \x00 as null char.
	// Let's just use DecodeGBK which trims right, and manually trim left if needed?
	// Actually, standard DecodeGBK trims right \x00. If it was padded left, the string starts with \x00.
	// We should trim left \x00 as well for this specific field.
	ip, _ := jtt809.DecodeGBK(body[1:33])
	ip = strings.TrimLeft(ip, "\x00")

	ack := RealTimeVideoStartupAck{
		Result:     body[0],
		ServerIP:   ip,
		ServerPort: binary.BigEndian.Uint16(body[33:35]),
	}
	return ack, nil
}
