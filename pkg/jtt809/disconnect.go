package jtt809

import "errors"

// LogoutRequest 主链路注销请求（0x1003），由下级主动发起。
type LogoutRequest struct {
	UserID   uint32
	Password string // 长度8
}

func (LogoutRequest) MsgID() uint16 { return MsgIDLogoutRequest }

func (l LogoutRequest) Encode() ([]byte, error) {
	return LoginRequest{UserID: l.UserID, Password: l.Password, DownLinkIP: "0", DownLinkPort: 0}.Encode()
}

// LogoutResponse 主链路注销应答（0x1004），业务体为空。
type LogoutResponse struct{}

func (LogoutResponse) MsgID() uint16           { return MsgIDLogoutResponse }
func (LogoutResponse) Encode() ([]byte, error) { return []byte{}, nil }

// DisconnectInform 主链路断开通知（0x1007），从链路下发，用于告知登录失效或链路异常原因。
type DisconnectInform struct {
	ErrorCode byte
}

func (DisconnectInform) MsgID() uint16 { return MsgIDDisconnNotify }
func (d DisconnectInform) Encode() ([]byte, error) {
	return []byte{d.ErrorCode}, nil
}

// ParseDisconnectInform 解析主链路断开通知，校验业务 ID 与载荷长度。
func ParseDisconnectInform(frame *Frame) (*DisconnectInform, error) {
	if frame == nil {
		return nil, errors.New("frame is nil")
	}
	if frame.BodyID != MsgIDDisconnNotify {
		return nil, errors.New("unexpected body id")
	}
	if len(frame.RawBody) < 1 {
		return nil, errors.New("body too short")
	}
	return &DisconnectInform{ErrorCode: frame.RawBody[0]}, nil
}

// SubLinkCloseNotify 上级主动关闭从链路通知（0x9008），下级收到后可释放连接资源。
type SubLinkCloseNotify struct {
	ReasonCode byte
}

func (SubLinkCloseNotify) MsgID() uint16             { return MsgIDCloseNotify }
func (s SubLinkCloseNotify) Encode() ([]byte, error) { return []byte{s.ReasonCode}, nil }

// BuildLogoutRequestPackage 便捷构造注销请求完整报文（含转义）。
func BuildLogoutRequestPackage(header Header, req LogoutRequest) ([]byte, error) {
	header.BusinessType = MsgIDLogoutRequest
	return EncodePackage(Package{
		Header: header,
		Body:   req,
	})
}

// BuildLogoutResponsePackage 便捷构造注销应答完整报文（含转义），根据请求头反填应答业务 ID。
func BuildLogoutResponsePackage(requestHeader Header) ([]byte, error) {
	header := requestHeader.WithResponse(MsgIDLogoutResponse)
	return EncodePackage(Package{
		Header: header,
		Body:   LogoutResponse{},
	})
}
