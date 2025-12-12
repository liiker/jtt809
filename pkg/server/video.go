package server

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/zboyco/jtt809/pkg/jtt809"
	"github.com/zboyco/jtt809/pkg/jtt809/jt1078"
)

var (
	ErrAuthCodeNotAvailable = errors.New("下级平台未返回授权码")
	ErrNoVideoResponse      = errors.New("下级平台未返回实时视频应答")
	ErrVideoNotAccepted     = errors.New("下级平台拒绝实时视频请求")
	ErrVideoServerMissing   = errors.New("下级平台未返回实时视频服务地址")
)

// VideoRequest 表示向下级平台下发的实时音视频请求。
type VideoRequest struct {
	UserID       uint32            `json:"user_id"`
	VehicleNo    string            `json:"vehicle_no"`
	VehicleColor jtt809.PlateColor `json:"vehicle_color"`
	ChannelID    byte              `json:"channel_id"`
	AVItemType   byte              `json:"av_item_type"`
	GnssHex      string            `json:"gnss_hex,omitempty"`
}

// RequestVideoStreamByPlate 仅通过车牌与颜色发起实时视频请求。
// 内部自动查找车辆归属的平台，复用 RequestVideoStream 的发送逻辑。
func (g *JT809Gateway) RequestVideoStreamByPlate(plate string, color jtt809.PlateColor, channelID byte, avItemType byte, gnssHex string) error {
	snap, _, err := g.findVehicleSnapshot(plate, color)
	if err != nil {
		return err
	}
	return g.RequestVideoStream(VideoRequest{
		UserID:       snap.UserID,
		VehicleNo:    plate,
		VehicleColor: color,
		ChannelID:    channelID,
		AVItemType:   avItemType,
		GnssHex:      gnssHex,
	})
}

// VideoStreamUrlByPlate 通过车牌与颜色返回 jtt1078 RTP 拉流地址。
// URL 形如 http://${serverIP}:${serverPort}/${plate}.${color}.${channel}.${avFlag}.${authCode}
// channel: 通道号
// avFlag: 0: 音视频; 1: 只音频; 2: 只视频
func (g *JT809Gateway) VideoStreamUrlByPlate(plate string, color jtt809.PlateColor, channelID int, avFlag int) (string, error) {
	snap, vehicle, err := g.findVehicleSnapshot(plate, color)
	if err != nil {
		return "", err
	}
	if snap.AuthCode == "" {
		return "", ErrAuthCodeNotAvailable
	}
	if vehicle.LastVideoAck == nil {
		return "", ErrNoVideoResponse
	}
	if vehicle.LastVideoAck.Result != 0 {
		return "", ErrVideoNotAccepted
	}
	if vehicle.LastVideoAck.ServerIP == "" || vehicle.LastVideoAck.ServerPort == 0 {
		return "", ErrVideoServerMissing
	}
	return fmt.Sprintf("http://%s:%d/%s.%d.%d.%d.%s",
		vehicle.LastVideoAck.ServerIP,
		vehicle.LastVideoAck.ServerPort,
		vehicle.VehicleNo,
		vehicle.VehicleColor,
		channelID,
		avFlag,
		snap.AuthCode,
	), nil
}

// RequestVideoStream 通过从链路向下级平台发送实时视频请求（0x9801 下行实时音视频）。
// 注意：0x9801 是上级→下级的下行消息，应该通过从链路发送；
//
//	0x1801 是下级→上级的上行消息，通过主链路发送。
func (g *JT809Gateway) RequestVideoStream(req VideoRequest) error {
	if req.VehicleNo == "" {
		return errors.New("vehicle_no is required")
	}
	if req.VehicleColor == 0 {
		req.VehicleColor = jtt809.PlateColorBlue
	}
	_, authCode := g.store.GetAuthCode(req.UserID)
	if authCode == "" {
		return fmt.Errorf("authorize_code not found in store for platform %d. Please wait for the platform to report the authorize code after login", req.UserID)
	}
	snap, ok := g.store.Snapshot(req.UserID)
	if !ok {
		return fmt.Errorf("platform %d not online", req.UserID)
	}
	if snap.GNSSCenterID == 0 {
		return fmt.Errorf("gnss_center_id is missing for platform %d, abort send", req.UserID)
	}
	var (
		gnssData []byte
		err      error
	)
	if strings.TrimSpace(req.GnssHex) != "" {
		gnssData, err = hex.DecodeString(strings.TrimSpace(req.GnssHex))
		if err != nil {
			return fmt.Errorf("parse gnss hex: %w", err)
		}
		if len(gnssData) != 36 {
			return fmt.Errorf("gnss data must be 36 bytes, got %d", len(gnssData))
		}
	}
	body := jt1078.DownRealTimeVideoStartupReq{
		ChannelID:     req.ChannelID,
		AVItemType:    req.AVItemType,
		AuthorizeCode: authCode,
		GnssData:      gnssData,
	}
	payload, err := body.Encode()
	if err != nil {
		return fmt.Errorf("encode video request: %w", err)
	}
	subBody, err := buildSubBusinessBody(req.VehicleNo, req.VehicleColor, body.MsgID(), payload)
	if err != nil {
		return err
	}
	header := jtt809.Header{
		GNSSCenterID: snap.GNSSCenterID,
	}
	if err := g.SendToSubordinate(req.UserID, header, rawBody{
		msgID:   jtt809.MsgIDDownRealTimeVideo,
		payload: subBody,
	}); err != nil {
		return fmt.Errorf("send video request: %w", err)
	}
	slog.Info("video request sent", "user_id", req.UserID, "plate", req.VehicleNo, "channel", req.ChannelID)
	return nil
}

// rawBody 允许直接注入编码好的业务体。
type rawBody struct {
	msgID   uint16
	payload []byte
}

func (r rawBody) MsgID() uint16 { return r.msgID }

func (r rawBody) Encode() ([]byte, error) {
	return r.payload, nil
}

func buildSubBusinessBody(plate string, color jtt809.PlateColor, subID uint16, payload []byte) ([]byte, error) {
	plateBytes, err := jtt809.EncodeGBK(plate)
	if err != nil {
		return nil, fmt.Errorf("encode plate: %w", err)
	}
	buf := make([]byte, 0, 21+1+2+4+len(payload))
	field := make([]byte, 21)
	copy(field, plateBytes)
	buf = append(buf, field...)
	buf = append(buf, byte(color))
	var tmp [2]byte
	binary.BigEndian.PutUint16(tmp[:], subID)
	buf = append(buf, tmp[:]...)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(payload)))
	buf = append(buf, length...)
	buf = append(buf, payload...)
	return buf, nil
}

// findVehicleSnapshot 在本地缓存中按车牌和颜色查找车辆所在平台及车辆视图。
func (g *JT809Gateway) findVehicleSnapshot(plate string, color jtt809.PlateColor) (PlatformSnapshot, *VehicleSnapshot, error) {
	if strings.TrimSpace(plate) == "" {
		return PlatformSnapshot{}, nil, errors.New("plate is required")
	}
	if color == 0 {
		color = jtt809.PlateColorBlue
	}
	snapshots := g.store.Snapshots()
	for _, snap := range snapshots {
		for _, v := range snap.Vehicles {
			if v.VehicleNo == plate && v.VehicleColor == color {
				vehicle := v // 创建副本，避免引用循环变量
				return snap, &vehicle, nil
			}
		}
	}
	return PlatformSnapshot{}, nil, fmt.Errorf("vehicle %s with color %d not found", plate, color)
}
