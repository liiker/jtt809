package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/zboyco/jtt809/pkg/jtt809"
	"github.com/zboyco/jtt809/pkg/server"
)

// MarshalLogin 序列化登录事件
func MarshalLogin(userID uint32, req *jtt809.LoginRequest, resp *jtt809.LoginResponse) (EventType, []byte, error) {
	data := LoginEventData{
		UserID:       userID,
		DownLinkIP:   req.DownLinkIP,
		DownLinkPort: req.DownLinkPort,
		Result:       uint8(resp.Result),
	}
	return marshalEvent(EventTypeLogin, userID, data)
}

// MarshalVehicleRegistration 序列化车辆注册事件
func MarshalVehicleRegistration(userID uint32, plate string, color jtt809.PlateColor, reg *server.VehicleRegistration) (EventType, []byte, error) {
	data := VehicleRegistrationEventData{
		UserID:     userID,
		Plate:      plate,
		Color:      color,
		PlatformID: reg.PlatformID,
	}
	return marshalEvent(EventTypeVehicleRegistration, userID, data)
}

// MarshalVehicleLocation 序列化车辆定位事件
func MarshalVehicleLocation(userID uint32, plate string, color jtt809.PlateColor, pos *jtt809.VehiclePosition, gnss *jtt809.GNSSData) (EventType, []byte, error) {
	if gnss == nil {
		return "", nil, fmt.Errorf("gnss data is nil")
	}

	data := VehicleLocationEventData{
		UserID:    userID,
		Plate:     plate,
		Color:     color,
		Latitude:  gnss.Latitude,
		Longitude: gnss.Longitude,
		Altitude:  gnss.Altitude,
		Speed:     gnss.Speed,
		Direction: gnss.Direction,
		DateTime:  gnss.DateTime.Time().Format(time.RFC3339),
	}
	return marshalEvent(EventTypeVehicleLocation, userID, data)
}

// MarshalVehicleLocationSupplementary 序列化批量定位事件
func MarshalVehicleLocationSupplementary(userID uint32, plate string, color jtt809.PlateColor, gnss []jtt809.GNSSData) (EventType, []byte, error) {
	items := make([]GNSSDataItem, 0, len(gnss))
	for _, g := range gnss {
		items = append(items, GNSSDataItem{
			Latitude:  g.Latitude,
			Longitude: g.Longitude,
			Altitude:  g.Altitude,
			Speed:     g.Speed,
			Direction: g.Direction,
			DateTime:  g.DateTime.Time().Format(time.RFC3339),
		})
	}

	data := VehicleLocationSupplementaryEventData{
		UserID:  userID,
		Plate:   plate,
		Color:   color,
		Count:   len(gnss),
		Devices: items,
	}
	return marshalEvent(EventTypeVehicleLocationSupplementary, userID, data)
}

// MarshalVideoResponse 序列化视频应答事件
func MarshalVideoResponse(userID uint32, plate string, color jtt809.PlateColor, videoAck *server.VideoAckState) (EventType, []byte, error) {
	data := VideoResponseEventData{
		UserID:     userID,
		Plate:      plate,
		Color:      color,
		ServerIP:   videoAck.ServerIP,
		ServerPort: videoAck.ServerPort,
		Result:     videoAck.Result,
	}
	return marshalEvent(EventTypeVideoResponse, userID, data)
}

// MarshalAuthorize 序列化视频鉴权事件
func MarshalAuthorize(userID uint32, platformID string, authorizeCode string) (EventType, []byte, error) {
	data := AuthorizeEventData{
		UserID:        userID,
		PlatformID:    platformID,
		AuthorizeCode: authorizeCode,
	}
	return marshalEvent(EventTypeAuthorize, userID, data)
}

// MarshalMonitorStartupAck 序列化监控开启应答事件
func MarshalMonitorStartupAck(userID uint32, plate string, color jtt809.PlateColor) (EventType, []byte, error) {
	data := MonitorStartupAckEventData{
		UserID: userID,
		Plate:  plate,
		Color:  color,
	}
	return marshalEvent(EventTypeMonitorStartupAck, userID, data)
}

// MarshalMonitorEndAck 序列化监控结束应答事件
func MarshalMonitorEndAck(userID uint32, plate string, color jtt809.PlateColor) (EventType, []byte, error) {
	data := MonitorEndAckEventData{
		UserID: userID,
		Plate:  plate,
		Color:  color,
	}
	return marshalEvent(EventTypeMonitorEndAck, userID, data)
}

// MarshalWarnMsgAdptInfo 序列化报警信息事件
func MarshalWarnMsgAdptInfo(userID uint32, info *jtt809.WarnMsgAdptInfo) (EventType, []byte, error) {
	data := WarnMsgAdptInfoEventData{
		UserID:           userID,
		SourcePlatformID: info.SourcePlatformID,
		WarnType:         info.WarnType,
		WarnTime:         info.WarnTime.Format(time.RFC3339),
		VehicleNo:        info.VehicleNo,
		VehicleColor:     info.VehicleColor,
		InfoContent:      fmt.Sprintf("len=%d", info.InfoLength),
	}
	return marshalEvent(EventTypeWarnMsgAdptInfo, userID, data)
}

// marshalEvent 通用事件序列化
func marshalEvent(eventType EventType, userID uint32, data interface{}) (EventType, []byte, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return "", nil, fmt.Errorf("marshal data: %w", err)
	}

	event := Event{
		Type:      eventType,
		Timestamp: time.Now().Format(time.RFC3339),
		UserID:    userID,
		Data:      dataBytes,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return "", nil, fmt.Errorf("marshal event: %w", err)
	}

	return eventType, eventBytes, nil
}
