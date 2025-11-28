package jtt809

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// ApplyForMonitorStartup 启动车辆定位信息交换请求 (0x9200/0x9205)
type ApplyForMonitorStartup struct {
	VehicleNo    string
	VehicleColor byte
	ReasonCode   MonitorReasonCode
}

func (ApplyForMonitorStartup) MsgID() uint16 { return MsgIDDownExgMsg }

func (a ApplyForMonitorStartup) Encode() ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(PadRightGBK(a.VehicleNo, 21))
	buf.WriteByte(a.VehicleColor)
	_ = binary.Write(&buf, binary.BigEndian, SubMsgApplyForMonitorStartup)
	_ = binary.Write(&buf, binary.BigEndian, uint32(1)) // 数据长度=1
	buf.WriteByte(byte(a.ReasonCode))
	return buf.Bytes(), nil
}

// ApplyForMonitorEnd 结束车辆定位信息交换请求 (0x9200/0x9206)
type ApplyForMonitorEnd struct {
	VehicleNo    string
	VehicleColor byte
	ReasonCode   MonitorReasonCode
}

func (ApplyForMonitorEnd) MsgID() uint16 { return MsgIDDownExgMsg }

func (a ApplyForMonitorEnd) Encode() ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(PadRightGBK(a.VehicleNo, 21))
	buf.WriteByte(a.VehicleColor)
	_ = binary.Write(&buf, binary.BigEndian, SubMsgApplyForMonitorEnd)
	_ = binary.Write(&buf, binary.BigEndian, uint32(1)) // 数据长度=1
	buf.WriteByte(byte(a.ReasonCode))
	return buf.Bytes(), nil
}

// MonitorAckResult 车辆定位信息交换应答结果
type MonitorAckResult byte

const (
	MonitorAckSuccess MonitorAckResult = 0x00 // 成功
	MonitorAckFailure MonitorAckResult = 0x01 // 失败
)

// ParseMonitorAck 解析车辆定位信息交换应答 (0x1205/0x1206)
func ParseMonitorAck(payload []byte) (MonitorAckResult, error) {
	if len(payload) < 1 {
		return 0, errors.New("payload too short")
	}
	return MonitorAckResult(payload[0]), nil
}
