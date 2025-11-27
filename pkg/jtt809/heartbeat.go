package jtt809

import "time"

// HeartbeatRequest 表示主链路连接保持请求（0x1005），业务体为空。
type HeartbeatRequest struct{}

func (HeartbeatRequest) MsgID() uint16 { return MsgIDHeartbeatRequest }
func (HeartbeatRequest) Encode() ([]byte, error) {
	return []byte{}, nil
}

// HeartbeatResponse 表示主链路连接保持应答（0x1006），业务体为空。
type HeartbeatResponse struct{}

func (HeartbeatResponse) MsgID() uint16 { return MsgIDHeartbeatResponse }
func (HeartbeatResponse) Encode() ([]byte, error) {
	return []byte{}, nil
}

// StartHeartbeat 使用发送回调按固定周期发送心跳，适用于主链路或从链路的长连接保活。
// send 负责写出编码后的报文；stop 可随时关闭循环。
func StartHeartbeat(stop <-chan struct{}, interval time.Duration, header Header, send func([]byte) error) error {
	if send == nil {
		return ErrMissingSender
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return nil
		case <-ticker.C:
			frame, err := EncodePackage(Package{
				Header: header,
				Body:   HeartbeatRequest{},
			})
			if err != nil {
				return err
			}
			if err := send(frame); err != nil {
				return err
			}
		}
	}
}
