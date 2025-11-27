package jt1078

import (
	"bytes"
	"errors"

	"github.com/zboyco/jtt809/pkg/jtt809"
)

// AuthorizeStartupReq 对应 UP_AUTHORIZE_MSG_STARTUP_REQ (0x1701)
// 视频终端鉴权/启动信息上报
type AuthorizeStartupReq struct {
	PlatformID     string // 11 bytes
	AuthorizeCode1 string // 64 bytes
	AuthorizeCode2 string // 64 bytes
}

func (AuthorizeStartupReq) MsgID() uint16 { return jtt809.SubMsgAuthorizeStartupReq }

func (r AuthorizeStartupReq) Encode() ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(jtt809.PadRightGBK(r.PlatformID, 11))
	buf.Write(jtt809.PadRightGBK(r.AuthorizeCode1, 64))
	buf.Write(jtt809.PadRightGBK(r.AuthorizeCode2, 64))
	return buf.Bytes(), nil
}

func ParseAuthorizeStartupReq(body []byte) (AuthorizeStartupReq, error) {
	if len(body) < 11+64+64 {
		return AuthorizeStartupReq{}, errors.New("authorize startup body too short")
	}
	pid, _ := jtt809.DecodeGBK(body[0:11])
	ac1, _ := jtt809.DecodeGBK(body[11:75])
	ac2, _ := jtt809.DecodeGBK(body[75:139])
	req := AuthorizeStartupReq{
		PlatformID:     pid,
		AuthorizeCode1: ac1,
		AuthorizeCode2: ac2,
	}
	return req, nil
}
