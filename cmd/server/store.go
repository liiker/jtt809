package main

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/zboyco/jtt809/pkg/jtt809"
)

// PlatformStore 维护下级平台与车辆状态。
type PlatformStore struct {
	mu           sync.RWMutex
	platforms    map[uint32]*PlatformState
	sessionIndex map[string]uint32
}

// PlatformState 表示单个下级平台的会话信息与车辆缓存。
type PlatformState struct {
	UserID        uint32
	GNSSCenterID  uint32
	DownLinkIP    string
	DownLinkPort  uint16
	MainSessionID string
	SubConn       net.Conn

	LastMainHeartbeat time.Time
	LastSubHeartbeat  time.Time

	Vehicles map[string]*VehicleState
}

// VehicleState 保存车辆注册信息、最新定位与最后一次视频应答。
type VehicleState struct {
	Number string
	Color  byte

	Registration *VehicleRegistration

	Position2011 *jtt809.VehiclePosition
	Position2019 *jtt809.VehiclePosition2019
	PositionTime time.Time
	BatchCount   int

	LastVideoAck *VideoAckState
}

// VehicleRegistration 描述车辆注册上报内容。
type VehicleRegistration struct {
	PlatformID        string
	ProducerID        string
	TerminalModelType string
	IMEI              string
	TerminalID        string
	TerminalSIM       string
	ReceivedAt        time.Time
}

// VideoAckState 表示下级平台返回的视频流地址信息。
type VideoAckState struct {
	Result     byte
	ServerIP   string
	ServerPort uint16
	ReceivedAt time.Time
}

// PlatformSnapshot 用于对外展示平台及车辆状态。
type PlatformSnapshot struct {
	UserID        uint32            `json:"user_id"`
	GNSSCenterID  uint32            `json:"gnss_center_id"`
	DownLinkIP    string            `json:"down_link_ip"`
	DownLinkPort  uint16            `json:"down_link_port"`
	MainSessionID string            `json:"main_session_id"`
	SubConnected  bool              `json:"sub_connected"`
	LastMainBeat  time.Time         `json:"last_main_heartbeat"`
	LastSubBeat   time.Time         `json:"last_sub_heartbeat"`
	Vehicles      []VehicleSnapshot `json:"vehicles"`
}

// VehicleSnapshot 为单车数据提供可序列化视图。
type VehicleSnapshot struct {
	VehicleNo    string                      `json:"vehicle_no"`
	VehicleColor byte                        `json:"vehicle_color"`
	Registration *VehicleRegistration        `json:"registration,omitempty"`
	Position2011 *jtt809.VehiclePosition     `json:"location_2011,omitempty"`
	Position2019 *jtt809.VehiclePosition2019 `json:"location_2019,omitempty"`
	PositionTime time.Time                   `json:"location_time,omitempty"`
	BatchCount   int                         `json:"batch_count,omitempty"`
	LastVideoAck *VideoAckState              `json:"video_ack,omitempty"`
}

// NewPlatformStore 初始化状态存储。
func NewPlatformStore() *PlatformStore {
	return &PlatformStore{
		platforms:    make(map[uint32]*PlatformState),
		sessionIndex: make(map[string]uint32),
	}
}

// BindMainSession 在主链路登录成功后建立会话映射。
func (s *PlatformStore) BindMainSession(sessionID string, req jtt809.LoginRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.ensurePlatformLocked(req.UserID)
	// 注意：LoginRequest 中已无 GNSSCenterID，该信息在 Header 中
	// 这里我们假设 Header 中的 GNSSCenterID 已经校验过，或者直接信任
	// if header.GNSSCenterID != s.centerID { ... }
	state.DownLinkIP = req.DownLinkIP
	state.DownLinkPort = req.DownLinkPort
	state.MainSessionID = sessionID
	state.LastMainHeartbeat = time.Now()
	s.sessionIndex[sessionID] = req.UserID
}

// BindSubSession 记录从链路连接。
func (s *PlatformStore) BindSubSession(userID uint32, conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.ensurePlatformLocked(userID)
	if state.SubConn != nil {
		state.SubConn.Close()
	}
	state.SubConn = conn
	state.LastSubHeartbeat = time.Now()
}

// RecordHeartbeat 更新心跳时间。
func (s *PlatformStore) RecordHeartbeat(userID uint32, isMain bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.ensurePlatformLocked(userID)
	if isMain {
		state.LastMainHeartbeat = time.Now()
	} else {
		state.LastSubHeartbeat = time.Now()
	}
}

// RemoveSession 在连接关闭时清理索引。
func (s *PlatformStore) RemoveSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	userID, ok := s.sessionIndex[sessionID]
	if !ok {
		return
	}
	state := s.platforms[userID]
	if state == nil {
		delete(s.sessionIndex, sessionID)
		return
	}
	if state.MainSessionID == sessionID {
		state.MainSessionID = ""
	}
	if state.SubConn != nil {
		// 这里我们无法通过 sessionID 判断是哪个 subConn，因为 subConn 没有 sessionID
		// 但 RemoveSession 是由 go-server 回调触发的，通常只针对 MainSession (因为 SubSrv 被移除了)
		// 所以这里只需要处理 MainSessionID
	}
	delete(s.sessionIndex, sessionID)
}

// UpdateVehicleRegistration 存储车辆注册信息。
func (s *PlatformStore) UpdateVehicleRegistration(userID uint32, color byte, vehicle string, reg *VehicleRegistration) {
	if reg == nil {
		return
	}
	reg.ReceivedAt = time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.ensurePlatformLocked(userID)
	v := state.ensureVehicleLocked(vehicleKey(vehicle, color), vehicle, color)
	v.Registration = reg
}

// UpdateLocation 写入最新定位数据，兼容 2011 与 2019 载荷。
func (s *PlatformStore) UpdateLocation(userID uint32, color byte, vehicle string, pos2011 *jtt809.VehiclePosition, pos2019 *jtt809.VehiclePosition2019, batchCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.ensurePlatformLocked(userID)
	v := state.ensureVehicleLocked(vehicleKey(vehicle, color), vehicle, color)
	if pos2011 != nil {
		cp := *pos2011
		v.Position2011 = &cp
		v.Position2019 = nil
		v.PositionTime = cp.Time
	} else if pos2019 != nil {
		cp := *pos2019
		v.Position2019 = &cp
		v.Position2011 = nil
		v.PositionTime = time.Now()
	}
	if batchCount > 0 {
		v.BatchCount = batchCount
	}
}

// RecordVideoAck 缓存最新视频流地址。
func (s *PlatformStore) RecordVideoAck(userID uint32, color byte, vehicle string, ack *VideoAckState) {
	if ack == nil {
		return
	}
	ack.ReceivedAt = time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.ensurePlatformLocked(userID)
	v := state.ensureVehicleLocked(vehicleKey(vehicle, color), vehicle, color)
	v.LastVideoAck = ack
}

// Snapshot 返回指定 userID 的深拷贝视图。
func (s *PlatformStore) Snapshot(userID uint32) (PlatformSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.platforms[userID]
	if !ok {
		return PlatformSnapshot{}, false
	}
	return state.snapshotLocked(), true
}

// Snapshots 列出所有平台状态。
func (s *PlatformStore) Snapshots() []PlatformSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]PlatformSnapshot, 0, len(s.platforms))
	for _, st := range s.platforms {
		result = append(result, st.snapshotLocked())
	}
	return result
}

// PlatformForSession 返回 sessionID 对应的平台 ID。
func (s *PlatformStore) PlatformForSession(sessionID string) (uint32, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.sessionIndex[sessionID]
	return user, ok
}

func (s *PlatformStore) ensurePlatformLocked(userID uint32) *PlatformState {
	state, ok := s.platforms[userID]
	if ok {
		return state
	}
	state = &PlatformState{
		UserID:   userID,
		Vehicles: make(map[string]*VehicleState),
	}
	s.platforms[userID] = state
	return state
}

func (state *PlatformState) ensureVehicleLocked(key string, number string, color byte) *VehicleState {
	v, ok := state.Vehicles[key]
	if ok {
		return v
	}
	v = &VehicleState{
		Number: number,
		Color:  color,
	}
	state.Vehicles[key] = v
	return v
}

func (state *PlatformState) snapshotLocked() PlatformSnapshot {
	snap := PlatformSnapshot{
		UserID:        state.UserID,
		GNSSCenterID:  state.GNSSCenterID,
		DownLinkIP:    state.DownLinkIP,
		DownLinkPort:  state.DownLinkPort,
		MainSessionID: state.MainSessionID,
		SubConnected:  state.SubConn != nil,
		LastMainBeat:  state.LastMainHeartbeat,
		LastSubBeat:   state.LastSubHeartbeat,
		Vehicles:      make([]VehicleSnapshot, 0, len(state.Vehicles)),
	}
	for _, v := range state.Vehicles {
		vs := VehicleSnapshot{
			VehicleNo:    v.Number,
			VehicleColor: v.Color,
			BatchCount:   v.BatchCount,
			PositionTime: v.PositionTime,
		}
		if v.Registration != nil {
			cp := *v.Registration
			vs.Registration = &cp
		}
		if v.Position2011 != nil {
			cp := *v.Position2011
			vs.Position2011 = &cp
		}
		if v.Position2019 != nil {
			cp := *v.Position2019
			vs.Position2019 = &cp
		}
		if v.LastVideoAck != nil {
			cp := *v.LastVideoAck
			vs.LastVideoAck = &cp
		}
		snap.Vehicles = append(snap.Vehicles, vs)
	}
	return snap
}

func vehicleKey(no string, color byte) string {
	return fmt.Sprintf("%s#%d", no, color)
}
