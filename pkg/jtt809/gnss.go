package jtt809

import (
	"encoding/binary"
	"fmt"
	"time"
)

// GNSSData 表示 36 字节 GNSS 数据。
type GNSSData struct {
	Encrypt     byte
	DateTime    GNSSTime
	Longitude   float64
	Latitude    float64
	Speed       uint16 // 车辆速度（0.1 km/h）
	RecordSpeed uint16 // 行驶记录仪速度（0.1 km/h）
	Mileage     uint32 // 单位 0.1 km
	Direction   uint16 // 0-359°
	Altitude    uint16 // 单位 m
	State       uint32 // 车辆状态位
	Alarm       uint32 // 报警标志
}

// GNSSTime 表示 GNSS 数据内的日期时间字段。
type GNSSTime struct {
	Year   uint16
	Month  byte
	Day    byte
	Hour   byte
	Minute byte
	Second byte
}

// Time 返回 time.Time，默认使用 UTC 时区。如果字段非法则返回零值。
func (t GNSSTime) Time(loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	if t.Month == 0 || t.Month > 12 || t.Day == 0 || t.Day > 31 {
		return time.Time{}
	}
	return time.Date(int(t.Year), time.Month(t.Month), int(t.Day),
		int(t.Hour), int(t.Minute), int(t.Second), 0, loc)
}

// ParseGNSSData 解码 36 字节 GNSS 数据。
func ParseGNSSData(data []byte) (GNSSData, error) {
	if len(data) < 36 {
		return GNSSData{}, fmt.Errorf("gnss payload too short: %d", len(data))
	}
	gnss := GNSSData{
		Encrypt: data[0],
		DateTime: GNSSTime{
			Day:    data[1],
			Month:  data[2],
			Year:   binary.BigEndian.Uint16(data[3:5]),
			Hour:   data[5],
			Minute: data[6],
			Second: data[7],
		},
		Speed:       binary.BigEndian.Uint16(data[16:18]),
		RecordSpeed: binary.BigEndian.Uint16(data[18:20]),
		Mileage:     binary.BigEndian.Uint32(data[20:24]),
		Direction:   binary.BigEndian.Uint16(data[24:26]),
		Altitude:    binary.BigEndian.Uint16(data[26:28]),
		State:       binary.BigEndian.Uint32(data[28:32]),
		Alarm:       binary.BigEndian.Uint32(data[32:36]),
	}
	gnss.Longitude = float64(int32(binary.BigEndian.Uint32(data[8:12]))) / 1e6
	gnss.Latitude = float64(int32(binary.BigEndian.Uint32(data[12:16]))) / 1e6
	return gnss, nil
}
