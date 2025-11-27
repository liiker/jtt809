package jtt809

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
)

// VehiclePosition 表示 2011 版车辆定位数据体，对应实时/补报定位子业务。
type VehiclePosition struct {
	Encrypt     byte
	Time        time.Time
	Lon         uint32 // 1e-6度
	Lat         uint32 // 1e-6度
	Speed       uint16 // km/h
	RecordSpeed uint16
	Mileage     uint32 // km
	Direction   uint16 // 0-359
	Altitude    uint16 // m
	State       uint32
	Alarm       uint32
}

// VehiclePosition2019 表示 2019 版车辆定位扩展，携带 GNSS 原始数据与多平台报警信息。
type VehiclePosition2019 struct {
	Encrypt     byte
	GnssData    []byte
	PlatformID1 string // 11字节，不足补0
	Alarm1      uint32
	PlatformID2 string // 11字节
	Alarm2      uint32
	PlatformID3 string // 11字节
	Alarm3      uint32
}

// Validate 校验经纬度、方向与时间等必填字段，确保定位数据合法。
func (v VehiclePosition) Validate() error {
	if v.Lon > 180000000 || v.Lat > 90000000 {
		return fmt.Errorf("invalid lon/lat: %d/%d", v.Lon, v.Lat)
	}
	if v.Direction > 359 {
		return fmt.Errorf("invalid direction: %d", v.Direction)
	}
	if v.Time.IsZero() {
		return errors.New("timestamp is required")
	}
	return nil
}

func (v VehiclePosition) encode() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteByte(v.Encrypt)
	buf.WriteByte(byte(v.Time.Day()))
	buf.WriteByte(byte(v.Time.Month()))
	_ = binary.Write(&buf, binary.BigEndian, uint16(v.Time.Year()))
	buf.WriteByte(byte(v.Time.Hour()))
	buf.WriteByte(byte(v.Time.Minute()))
	buf.WriteByte(byte(v.Time.Second()))
	_ = binary.Write(&buf, binary.BigEndian, v.Lon)
	_ = binary.Write(&buf, binary.BigEndian, v.Lat)
	_ = binary.Write(&buf, binary.BigEndian, v.Speed)
	_ = binary.Write(&buf, binary.BigEndian, v.RecordSpeed)
	_ = binary.Write(&buf, binary.BigEndian, v.Mileage)
	_ = binary.Write(&buf, binary.BigEndian, v.Direction)
	_ = binary.Write(&buf, binary.BigEndian, v.Altitude)
	_ = binary.Write(&buf, binary.BigEndian, v.State)
	_ = binary.Write(&buf, binary.BigEndian, v.Alarm)
	return buf.Bytes(), nil
}

func (v VehiclePosition2019) encode() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(v.Encrypt)
	pos := buf.Len()
	_ = binary.Write(&buf, binary.BigEndian, uint32(0)) // 占位长度
	buf.Write(v.GnssData)
	// 回写长度
	l := uint32(len(v.GnssData))
	binary.BigEndian.PutUint32(buf.Bytes()[pos:], l)
	buf.Write(PadRightGBK(v.PlatformID1, 11))
	_ = binary.Write(&buf, binary.BigEndian, v.Alarm1)
	buf.Write(PadRightGBK(v.PlatformID2, 11))
	_ = binary.Write(&buf, binary.BigEndian, v.Alarm2)
	buf.Write(PadRightGBK(v.PlatformID3, 11))
	_ = binary.Write(&buf, binary.BigEndian, v.Alarm3)
	return buf.Bytes(), nil
}

// VehicleLocationUpload 表示主链路车辆动态信息交换（0x1200）业务体，承载实时定位数据（支持 2011 与 2019 版载荷，二选一）。
type VehicleLocationUpload struct {
	VehicleNo    string
	VehicleColor byte
	Position     VehiclePosition
	Position2019 *VehiclePosition2019
}

func (VehicleLocationUpload) MsgID() uint16 { return MsgIDDynamicInfo }

func (v VehicleLocationUpload) Encode() ([]byte, error) {
	if len(v.VehicleNo) == 0 {
		return nil, errors.New("vehicle number is required")
	}
	var (
		positionBody []byte
		err          error
	)
	switch {
	case v.Position2019 != nil:
		positionBody, err = v.Position2019.encode()
	default:
		positionBody, err = v.Position.encode()
	}
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(PadRightGBK(v.VehicleNo, 21))
	buf.WriteByte(v.VehicleColor)

	const subMsgID uint16 = 0x1202
	_ = binary.Write(&buf, binary.BigEndian, subMsgID)

	_ = binary.Write(&buf, binary.BigEndian, uint32(len(positionBody)))
	buf.Write(positionBody)
	return buf.Bytes(), nil
}

// ParseVehiclePosition 反序列化 2011 版车辆定位载荷，便于测试或上层解析。
func ParseVehiclePosition(body []byte) (VehiclePosition, error) {
	if len(body) < 36 {
		return VehiclePosition{}, errors.New("position body too short")
	}
	pos := VehiclePosition{
		Encrypt: body[0],
		Time: time.Date(int(binary.BigEndian.Uint16(body[3:5])),
			time.Month(body[2]), int(body[1]),
			int(body[5]), int(body[6]), int(body[7]), 0, time.UTC),
		Lon:         binary.BigEndian.Uint32(body[8:12]),
		Lat:         binary.BigEndian.Uint32(body[12:16]),
		Speed:       binary.BigEndian.Uint16(body[16:18]),
		RecordSpeed: binary.BigEndian.Uint16(body[18:20]),
		Mileage:     binary.BigEndian.Uint32(body[20:24]),
		Direction:   binary.BigEndian.Uint16(body[24:26]),
		Altitude:    binary.BigEndian.Uint16(body[26:28]),
		State:       binary.BigEndian.Uint32(body[28:32]),
		Alarm:       binary.BigEndian.Uint32(body[32:36]),
	}
	return pos, nil
}

// ParseVehiclePosition2019 解析 2019 版定位载荷，保留原始 GNSS 数据，不做二次解码。
func ParseVehiclePosition2019(body []byte) (VehiclePosition2019, error) {
	if len(body) < 1+4+11+4+11+4+11+4 {
		return VehiclePosition2019{}, errors.New("position 2019 body too short")
	}
	pos := VehiclePosition2019{
		Encrypt: body[0],
	}
	dataLen := int(binary.BigEndian.Uint32(body[1:5]))
	if len(body) < 5+dataLen+11+4+11+4+11+4 {
		return VehiclePosition2019{}, errors.New("position 2019 body length mismatch")
	}
	pos.GnssData = append([]byte(nil), body[5:5+dataLen]...)
	offset := 5 + dataLen
	pos.PlatformID1 = strings.TrimRight(string(body[offset:offset+11]), "\x00")
	offset += 11
	pos.Alarm1 = binary.BigEndian.Uint32(body[offset : offset+4])
	offset += 4
	pos.PlatformID2 = strings.TrimRight(string(body[offset:offset+11]), "\x00")
	offset += 11
	pos.Alarm2 = binary.BigEndian.Uint32(body[offset : offset+4])
	offset += 4
	pos.PlatformID3 = strings.TrimRight(string(body[offset:offset+11]), "\x00")
	offset += 11
	pos.Alarm3 = binary.BigEndian.Uint32(body[offset : offset+4])
	return pos, nil
}
