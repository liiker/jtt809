package jtt809

import (
	"encoding/binary"
	"fmt"
	"time"
)

// GNSSData 表示车辆定位基础信息（表23）及附加信息（表26/27）。
type GNSSData struct {
	Alarm       uint32
	State       uint32
	Latitude    float64
	Longitude   float64
	Altitude    uint16 // 单位 m
	Speed       uint16 // 车辆速度（0.1 km/h）
	Direction   uint16 // 0-359°
	DateTime    GNSSTime
	Mileage     uint32            // 附加信息 0x01，单位 0.1 km
	Fuel        uint16            // 附加信息 0x02，单位 0.1 L
	RecordSpeed uint16            // 附加信息 0x03，行驶记录仪速度（0.1 km/h）
	Additional  map[byte][]byte   // 其他附加信息原始数据
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

func parseBCDByte(b byte) (int, error) {
	hi := b >> 4
	lo := b & 0x0F
	if hi >= 10 || lo >= 10 {
		return 0, fmt.Errorf("invalid BCD byte: 0x%02X", b)
	}
	return int(hi*10 + lo), nil
}

func parseBCDTime6(b []byte) (GNSSTime, error) {
	if len(b) < 6 {
		return GNSSTime{}, fmt.Errorf("invalid BCD time length: %d", len(b))
	}
	var parts [6]int
	for i := 0; i < 6; i++ {
		v, err := parseBCDByte(b[i])
		if err != nil {
			return GNSSTime{}, err
		}
		parts[i] = v
	}
	return GNSSTime{
		Year:   uint16(2000 + parts[0]),
		Month:  byte(parts[1]),
		Day:    byte(parts[2]),
		Hour:   byte(parts[3]),
		Minute: byte(parts[4]),
		Second: byte(parts[5]),
	}, nil
}

// ParseGNSSData 解码车辆定位基础信息（28 字节）及附加信息 TLV。
func ParseGNSSData(data []byte) (GNSSData, error) {
	const baseLen = 28
	if len(data) < baseLen {
		return GNSSData{}, fmt.Errorf("gnss payload too short: %d", len(data))
	}
	gnss := GNSSData{
		Alarm:     binary.BigEndian.Uint32(data[0:4]),
		State:     binary.BigEndian.Uint32(data[4:8]),
		Latitude:  float64(int32(binary.BigEndian.Uint32(data[8:12]))) / 1e6,
		Longitude: float64(int32(binary.BigEndian.Uint32(data[12:16]))) / 1e6,
		Altitude:  binary.BigEndian.Uint16(data[16:18]),
		Speed:     binary.BigEndian.Uint16(data[18:20]),
		Direction: binary.BigEndian.Uint16(data[20:22]),
	}

	t, err := parseBCDTime6(data[22:28])
	if err != nil {
		return GNSSData{}, fmt.Errorf("parse gnss time: %w", err)
	}
	gnss.DateTime = t

	if len(data) == baseLen {
		return gnss, nil
	}

	additional := make(map[byte][]byte)
	for idx := baseLen; idx < len(data); {
		if idx+2 > len(data) {
			return GNSSData{}, fmt.Errorf("gnss attachment header truncated at %d", idx)
		}
		id := data[idx]
		l := int(data[idx+1])
		idx += 2
		if idx+l > len(data) {
			return GNSSData{}, fmt.Errorf("gnss attachment %02X length %d exceeds payload", id, l)
		}
		val := append([]byte(nil), data[idx:idx+l]...)
		idx += l
		additional[id] = val

		switch id {
		case 0x01: // 里程
			if l >= 4 {
				gnss.Mileage = binary.BigEndian.Uint32(val[:4])
			}
		case 0x02: // 油量
			if l >= 2 {
				gnss.Fuel = binary.BigEndian.Uint16(val[:2])
			}
		case 0x03: // 行驶记录仪速度
			if l >= 2 {
				gnss.RecordSpeed = binary.BigEndian.Uint16(val[:2])
			}
		}
	}
	if len(additional) > 0 {
		gnss.Additional = additional
	}
	return gnss, nil
}
