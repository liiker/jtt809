package events

import (
	"encoding/json"

	"github.com/zboyco/jtt809/pkg/jtt809"
)

// EventType 事件类型定义
type EventType string

const (
	EventTypeLogin                    EventType = "login"
	EventTypeVehicleRegistration      EventType = "vehicle.registration"
	EventTypeVehicleLocation          EventType = "vehicle.location"
	EventTypeVehicleLocationSupplementary EventType = "vehicle.location.supplementary"
	EventTypeVideoResponse            EventType = "video.response"
	EventTypeAuthorize                EventType = "authorize"
	EventTypeMonitorStartupAck        EventType = "monitor.startup.ack"
	EventTypeMonitorEndAck            EventType = "monitor.end.ack"
	EventTypeWarnMsgAdptInfo          EventType = "warn.adpt.info"
)

// Event 事件基础结构
type Event struct {
	Type      EventType       `json:"type"`
	Timestamp string          `json:"timestamp"`
	UserID    uint32          `json:"user_id"`
	Data      json.RawMessage `json:"data"`
}

// LoginEventData 登录事件数据
type LoginEventData struct {
	UserID       uint32 `json:"user_id"`
	DownLinkIP   string `json:"down_link_ip"`
	DownLinkPort uint16 `json:"down_link_port"`
	Result       uint8  `json:"result"`
}

// VehicleRegistrationEventData 车辆注册事件数据
type VehicleRegistrationEventData struct {
	UserID     uint32            `json:"user_id"`
	Plate      string            `json:"plate"`
	Color      jtt809.PlateColor `json:"color"`
	PlatformID string            `json:"platform_id"`
	// 具体车辆注册数据将由 VehicleRegistration 结构体提供
}

// VehicleLocationEventData 车辆定位事件数据
type VehicleLocationEventData struct {
	UserID    uint32            `json:"user_id"`
	Plate     string            `json:"plate"`
	Color     jtt809.PlateColor `json:"color"`
	Latitude  float64           `json:"latitude"`
	Longitude float64           `json:"longitude"`
	Altitude  uint16            `json:"altitude"`
	Speed     uint16            `json:"speed"`
	Direction uint16            `json:"direction"`
	DateTime  string            `json:"date_time"`
}

// VehicleLocationSupplementaryEventData 批量定位事件数据
type VehicleLocationSupplementaryEventData struct {
	UserID  uint32            `json:"user_id"`
	Plate   string            `json:"plate"`
	Color   jtt809.PlateColor `json:"color"`
	Count   int               `json:"count"`
	Devices []GNSSDataItem    `json:"devices,omitempty"`
}

// GNSSDataItem GNSS 定位数据项
type GNSSDataItem struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  uint16  `json:"altitude"`
	Speed     uint16  `json:"speed"`
	Direction uint16  `json:"direction"`
	DateTime  string  `json:"date_time"`
}

// VideoResponseEventData 视频应答事件数据
type VideoResponseEventData struct {
	UserID    uint32            `json:"user_id"`
	Plate     string            `json:"plate"`
	Color     jtt809.PlateColor `json:"color"`
	ServerIP  string            `json:"server_ip"`
	ServerPort uint16           `json:"server_port"`
	Result    uint8             `json:"result"`
	ChannelNo uint8             `json:"channel_no"`
}

// AuthorizeEventData 视频鉴权事件数据
type AuthorizeEventData struct {
	UserID        uint32 `json:"user_id"`
	PlatformID    string `json:"platform_id"`
	AuthorizeCode string `json:"authorize_code"`
}

// MonitorStartupAckEventData 监控开启应答事件数据
type MonitorStartupAckEventData struct {
	UserID uint32            `json:"user_id"`
	Plate  string            `json:"plate"`
	Color  jtt809.PlateColor `json:"color"`
}

// MonitorEndAckEventData 监控结束应答事件数据
type MonitorEndAckEventData struct {
	UserID uint32            `json:"user_id"`
	Plate  string            `json:"plate"`
	Color  jtt809.PlateColor `json:"color"`
}

// WarnMsgAdptInfoEventData 报警信息事件数据
type WarnMsgAdptInfoEventData struct {
	UserID           uint32              `json:"user_id"`
	SourcePlatformID string              `json:"source_platform_id"`
	WarnType         jtt809.WarnType     `json:"warn_type"`
	WarnTime         string              `json:"warn_time"`
	VehicleNo        string              `json:"vehicle_no"`
	VehicleColor     jtt809.PlateColor   `json:"vehicle_color"`
	InfoContent      string              `json:"info_content"`
}
