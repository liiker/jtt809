package jtt809

import "fmt"

// VehicleRegistrationInfo 表示 0x1201 车辆注册子业务中的注册详情。
type VehicleRegistrationInfo struct {
	PlatformID        string
	ProducerID        string
	TerminalModelType string
	IMEI              string
	TerminalID        string
	TerminalSIM       string
}

// ParseVehicleRegistration 解码 0x1201 注册载荷（固定长度）。
func ParseVehicleRegistration(payload []byte) (*VehicleRegistrationInfo, error) {
	const (
		lenPlatform = 11
		lenProducer = 11
		lenModel    = 30
		lenIMEI     = 15
		lenTermID   = 30
		lenSIM      = 13
	)
	total := lenPlatform + lenProducer + lenModel + lenIMEI + lenTermID + lenSIM
	if len(payload) < total {
		return nil, fmt.Errorf("registration payload too short: %d (expected %d)", len(payload), total)
	}
	offset := 0
	read := func(length int) []byte {
		v := payload[offset : offset+length]
		offset += length
		return v
	}
	platform, _ := DecodeGBK(read(lenPlatform))
	producer, _ := DecodeGBK(read(lenProducer))
	model, _ := DecodeGBK(read(lenModel))
	imei, _ := DecodeGBK(read(lenIMEI))
	terminalID, _ := DecodeGBK(read(lenTermID))
	sim, _ := DecodeGBK(read(lenSIM))
	return &VehicleRegistrationInfo{
		PlatformID:        platform,
		ProducerID:        producer,
		TerminalModelType: model,
		IMEI:              imei,
		TerminalID:        terminalID,
		TerminalSIM:       sim,
	}, nil
}
