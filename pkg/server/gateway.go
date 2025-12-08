package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	goserver "github.com/zboyco/go-server"
	"github.com/zboyco/go-server/client"
	"github.com/zboyco/jtt809/pkg/jtt809"
	"github.com/zboyco/jtt809/pkg/jtt809/jt1078"
)

// JT809Gateway 负责承载主/从链路 TCP 服务与业务处理。
type JT809Gateway struct {
	cfg   Config
	auth  *Authenticator
	store *PlatformStore

	mainSrv *goserver.Server
	httpSrv *http.Server

	callbacks *Callbacks // 消息回调

	startOnce sync.Once
}

func NewJT809Gateway(cfg Config) (*JT809Gateway, error) {
	if len(cfg.Accounts) == 0 {
		return nil, errors.New("at least one account is required")
	}

	printStartupInfo(cfg)

	return &JT809Gateway{
		cfg:   cfg,
		auth:  NewAuthenticator(cfg.Accounts),
		store: NewPlatformStore(),
	}, nil
}

// SetCallbacks 设置回调函数，用于在收到特定消息时执行自定义业务逻辑
func (g *JT809Gateway) SetCallbacks(callbacks *Callbacks) {
	g.callbacks = callbacks
}

// Start 同时启动主链路、从链路服务，并阻塞直至 ctx 结束。
func (g *JT809Gateway) Start(ctx context.Context) error {
	var startErr error
	g.startOnce.Do(func() {
		if err := g.initServers(); err != nil {
			startErr = err
			return
		}
		go g.mainSrv.Start()
		g.startHTTPServer(ctx)
		go g.healthCheckLoop(ctx)
	})
	if startErr != nil {
		return startErr
	}
	<-ctx.Done()
	slog.Info("gateway shutting down", "reason", ctx.Err())
	return nil
}

func (g *JT809Gateway) initServers() error {
	mainHost, mainPort, err := normalizeHostPort(g.cfg.MainListen)
	if err != nil {
		return fmt.Errorf("parse main listen: %w", err)
	}
	g.mainSrv = goserver.NewTCP(mainHost, mainPort)

	if g.cfg.IdleTimeout > 0 {
		g.mainSrv.IdleSessionTimeOut = int(g.cfg.IdleTimeout.Seconds())
	} else {
		g.mainSrv.IdleSessionTimeOut = 0
	}

	if err := g.mainSrv.SetSplitFunc(splitJT809Frames); err != nil {
		return err
	}

	if err := g.mainSrv.SetOnMessage(g.handleMainMessage); err != nil {
		return err
	}

	_ = g.mainSrv.SetOnError(func(err error) {
		slog.Error("main link error", "err", err)
	})

	_ = g.mainSrv.SetOnSessionClosed(g.onSessionClosed)
	_ = g.mainSrv.SetOnNewSessionRegister(func(s *goserver.AppSession) {
		ip := g.getClientIP(s)
		if ip != "" {
			slog.Info("main link connected", "session", s.ID, "remote_ip", ip)
			return
		}
		slog.Info("main link connected", "session", s.ID)
	})
	return nil
}

// handleMainMessage 处理主链路报文。
func (g *JT809Gateway) handleMainMessage(session *goserver.AppSession, payload []byte) ([]byte, error) {
	g.logPacket("main", "recv", session.ID, payload)
	frame, err := jtt809.DecodeFrame(payload)
	if err != nil {
		slog.Warn("decode main frame failed", "session", session.ID, "err", err)
		return nil, nil
	}
	if _, ok := g.sessionUser(session); !ok && frame.BodyID != jtt809.MsgIDLoginRequest {
		// 未登录成功前的报文直接忽略
		slog.Warn("ignore message before login", "session", session.ID, "msg_id", fmt.Sprintf("0x%04X", frame.BodyID))
		return nil, nil
	}
	switch frame.BodyID {
	case jtt809.MsgIDLoginRequest:
		return g.handleMainLogin(session, frame)
	case jtt809.MsgIDHeartbeatRequest:
		return g.handleHeartbeat(session, frame, true)
	case jtt809.MsgIDLogoutRequest:
		return g.simpleResponse(session, "main", frame, jtt809.LogoutResponse{})
	case jtt809.MsgIDDownDisconnectInform:
		g.handleDisconnectInform(session, frame)
	default:
		user, ok := g.sessionUser(session)
		if ok {
			g.handleBusinessMessage(user, frame, true)
		}
	}
	return nil, nil
}

// handleSubMessage 处理从链路报文（Active Mode）。
// 正常情况下从链路用于接收应答，但当主链路断开时，下级平台可能通过从链路发送请求。
func (g *JT809Gateway) handleSubMessage(userID uint32, payload []byte) {
	g.logPacket("sub", "recv", fmt.Sprintf("%d", userID), payload)
	frame, err := jtt809.DecodeFrame(payload)
	if err != nil {
		slog.Warn("decode sub frame failed", "user_id", userID, "err", err)
		return
	}

	switch frame.BodyID {
	case jtt809.MsgIDDownlinkConnReq:
		slog.Debug("received sub link login request on sub link", "user_id", userID)
	case jtt809.MsgIDDownlinkConnResp:
		slog.Debug("received sub link login response", "user_id", userID)
	case jtt809.MsgIDDisconnNotify:
		g.handleSubDisconnect(userID, frame)
	default:
		g.handleBusinessMessage(userID, frame, false)
	}
}

// handleBusinessMessage 处理可能在主链路或从链路接收的业务消息
func (g *JT809Gateway) handleBusinessMessage(userID uint32, frame *jtt809.Frame, receivedOnMain bool) {
	switch frame.BodyID {
	case jtt809.MsgIDDynamicInfo:
		g.handleDynamicInfo(userID, frame)
	case jtt809.MsgIDPlatformInfo:
		g.handlePlatformInfo(userID, frame)
	case jtt809.MsgIDRealTimeVideo:
		g.handleRealTimeVideo(userID, frame)
	case jtt809.MsgIDAuthorize:
		g.handleAuthorize(userID, frame)
	case jtt809.MsgIDAlarmInteract:
		g.handleAlarmInteract(nil, frame)
	case jtt809.MsgIDDownHeartbeatResponse:
		g.store.RecordHeartbeat(userID, false)
	default:
		linkType := "main"
		if !receivedOnMain {
			linkType = "sub"
		}
		slog.Debug("unhandled business message", "user_id", userID, "link", linkType, "msg_id", fmt.Sprintf("0x%04X", frame.BodyID))
	}
}

func (g *JT809Gateway) handleMainLogin(session *goserver.AppSession, frame *jtt809.Frame) ([]byte, error) {
	req, err := jtt809.ParseLoginRequest(frame.RawBody)
	if err != nil {
		slog.Warn("parse main login failed", "session", session.ID, "err", err)
		return nil, nil
	}
	clientIP := g.getClientIP(session)
	acc, resp := g.auth.Authenticate(req, clientIP)
	slog.Info("main login request", "session", session.ID, "user_id", req.UserID, "gnss", frame.Header.GNSSCenterID, "ip", clientIP, "result", resp.Result)
	if resp.Result == jtt809.LoginOK {
		session.SetAttr("userID", req.UserID)
		session.SetAttr("link", "main")
		g.store.BindMainSession(session.ID, req, acc.GnssCenterID, resp.VerifyCode)

		// 触发登录回调
		if g.callbacks != nil && g.callbacks.OnLogin != nil {
			go g.callbacks.OnLogin(req.UserID, &req, &resp)
		}

		// Start Sub Link Connection
		go g.connectSubLinkWithRetry(req.UserID, false)
	}
	// 主链路登录应答应该在主链路返回（使用相同链路）
	data, respErr := g.simpleResponse(session, "main", frame, resp)
	if resp.Result != jtt809.LoginOK {
		// 登录失败后立即断开
		session.Close("login failed")
	}
	return data, respErr
}

func (g *JT809Gateway) connectSubLinkWithRetry(userID uint32, isReconnect bool) {
	// 设置重连标志，如果已经在重连则直接返回
	if !g.store.SetReconnecting(userID, true) {
		slog.Info("sub link already reconnecting, skip", "user_id", userID)
		return
	}
	defer g.store.SetReconnecting(userID, false)

	// 重试次数策略
	maxRetries := 3
	if !isReconnect {
		maxRetries = 1
	}

	for i := 0; i < maxRetries; i++ {
		// 检查从链路是否已连接（防止竞态条件导致重复连接）
		_, subActive := g.store.GetLinkStatus(userID)
		if subActive {
			slog.Info("sub link already connected, stop reconnecting", "user_id", userID)
			return
		}

		// 检查主链路是否仍然活跃
		snap, ok := g.store.Snapshot(userID)
		if !ok || snap.MainSessionID == "" {
			slog.Info("main link not active, stop reconnecting", "user_id", userID)
			return
		}

		if snap.DownLinkIP == "" || snap.DownLinkPort == 0 {
			slog.Warn("missing sub link address, stop reconnecting", "user_id", userID, "ip", snap.DownLinkIP, "port", snap.DownLinkPort)
			return
		}
		if snap.GNSSCenterID == 0 {
			slog.Warn("missing GNSSCenterID, stop sub link reconnect", "user_id", userID)
			return
		}

		if g.connectSubLink(snap.DownLinkIP, snap.DownLinkPort, userID, snap.GNSSCenterID, snap.VerifyCode) {
			return
		}

		// 仅在重连模式下等待重试
		if isReconnect && i < maxRetries-1 {
			time.Sleep(10 * time.Second) // 缩短等待时间以便快速重试
			slog.Info("retrying sub link connection", "user_id", userID, "attempt", i+1)
		}
	}

	// 重试失败，发送从链路断开通知
	errorCode := jtt809.DisconnectCannotConnectSub // 情景2：无法连接指定IP端口 (0x00)
	if isReconnect {
		errorCode = jtt809.DisconnectSubLinkBroken // 情景1：重连三次失败 (0x01)
	}

	slog.Warn("sub link connection failed, sending notification", "user_id", userID, "error_code", errorCode)
	g.sendDownDisconnectInform(userID, errorCode)
}

// sendDownDisconnectInform 发送从链路断开通知 (0x9007)
func (g *JT809Gateway) sendDownDisconnectInform(userID uint32, code jtt809.DisconnectErrorCode) {
	snap, ok := g.store.Snapshot(userID)
	if !ok || snap.MainSessionID == "" {
		return
	}

	// 获取主链路 Session
	sessionID := snap.MainSessionID
	_, err := g.mainSrv.GetSessionByID(sessionID)
	if err != nil {
		slog.Warn("get main session failed for disconnect inform", "user_id", userID, "err", err)
		return
	}

	// 构造消息
	msg := jtt809.DownDisconnectInform{ErrorCode: code}
	// 注意：这里需要构造完整的 0x9007 报文，并复用 Header 信息（流水号自动生成）
	fakeHeader := jtt809.Header{
		GNSSCenterID: snap.GNSSCenterID,
		Version:      jtt809.Version{Major: 1, Minor: 0, Patch: 0}, // 使用默认或存储的版本
		EncryptFlag:  0,
		EncryptKey:   0,
	}

	// 发送
	if err := g.sendResponseOnLink(true, userID, &jtt809.Frame{Header: fakeHeader, BodyID: msg.MsgID()}, msg); err != nil {
		slog.Warn("send 0x9007 failed", "user_id", userID, "err", err)
	}
}

func (g *JT809Gateway) connectSubLink(ip string, port uint16, userID uint32, gnssCenterID uint32, verifyCode uint32) bool {
	slog.Info("connecting sub link", "ip", ip, "port", port, "user_id", userID)

	c := client.NewSimpleClient(goserver.TCP, ip, int(port))
	c.SetScannerSplitFunc(splitJT809Frames)

	if err := c.Connect(); err != nil {
		slog.Error("connect sub link failed", "err", err)
		return false
	}

	// 设置读写超时
	if conn := c.GetRawConn(); conn != nil {
		conn.SetDeadline(time.Now().Add(10 * time.Second))
	}

	// Send Login
	req := jtt809.SubLinkLoginRequest{VerifyCode: verifyCode}
	pkg, _ := jtt809.BuildSubLinkLoginPackage(jtt809.Header{
		BusinessType: jtt809.MsgIDDownlinkConnReq,
		GNSSCenterID: gnssCenterID,
	}, req)

	g.logPacket("sub", "send", fmt.Sprintf("%d", userID), pkg)
	if err := c.Send(pkg); err != nil {
		slog.Error("send sub login failed", "err", err)
		c.Close()
		return false
	}

	// Read Response
	respData, err := c.Receive()
	if err != nil {
		slog.Error("read sub login response failed", "err", err)
		c.Close()
		return false
	}
	g.logPacket("sub", "recv", fmt.Sprintf("%d", userID), respData)

	frame, err := jtt809.DecodeFrame(respData)
	if err != nil {
		slog.Error("decode sub login response failed", "err", err)
		c.Close()
		return false
	}

	if frame.BodyID != 0x9002 {
		slog.Error("unexpected sub login response", "msg_id", fmt.Sprintf("0x%04X", frame.BodyID))
		c.Close()
		return false
	}

	loginResp, err := jtt809.ParseSubLinkLoginResponse(frame)
	if err != nil {
		slog.Error("parse sub login response failed", "err", err)
		c.Close()
		return false
	}

	if loginResp.Result != 0 {
		slog.Error("sub link login refused", "result", loginResp.Result)
		c.Close()
		return false
	}

	slog.Info("sub link connected and logged in", "user_id", userID)

	// 清除超时限制，后续由心跳保活
	if conn := c.GetRawConn(); conn != nil {
		conn.SetDeadline(time.Time{})
	}

	// 创建 context 用于控制从链路相关 goroutine 的生命周期
	ctx, cancel := context.WithCancel(context.Background())
	g.store.BindSubSession(userID, c, ctx, cancel)

	go g.readSubLinkLoop(ctx, c, userID, gnssCenterID, verifyCode, true)
	go g.keepAliveSubLink(ctx, c, userID)
	return true
}

func (g *JT809Gateway) readSubLinkLoop(ctx context.Context, c *client.SimpleClient, userID uint32, gnssCenterID uint32, verifyCode uint32, shouldReconnect bool) {
	defer func() {
		c.Close()
		g.store.ClearSubConn(userID)
		slog.Info("sub link closed", "user_id", userID)
		// 仅在需要重连时触发
		if shouldReconnect {
			go g.reconnectSubLink(userID)
		}
	}()

	// 启用一个 goroutine 监听 context 取消
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		slog.Info("sub read loop stopping due to context cancel", "user_id", userID)
		c.Close() // 关闭连接以中断 Receive() 调用
		close(done)
	}()

	for {
		select {
		case <-done:
			return
		default:
			data, err := c.Receive()
			if err != nil {
				slog.Error("sub link read error", "user_id", userID, "err", err)
				return
			}
			g.handleSubMessage(userID, data)
		}
	}
}

func (g *JT809Gateway) reconnectSubLink(userID uint32) {
	time.Sleep(5 * time.Second)
	snap, ok := g.store.Snapshot(userID)
	if !ok || snap.MainSessionID == "" {
		slog.Info("skip sub link reconnect, main link not active", "user_id", userID)
		return
	}
	slog.Info("attempting sub link reconnect", "user_id", userID)
	g.connectSubLinkWithRetry(userID, true)
}

func (g *JT809Gateway) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.checkConnections()
		}
	}
}

func (g *JT809Gateway) checkConnections() {
	snapshots := g.store.Snapshots()
	now := time.Now()

	for _, snap := range snapshots {
		// 检查主链路断开超时情况
		if snap.MainSessionID == "" && snap.SubConnected && !snap.MainDisconnectedAt.IsZero() {
			// 主链路断开超过5分钟，关闭从链路以释放资源
			if now.Sub(snap.MainDisconnectedAt) > 5*time.Minute {
				slog.Warn("main link disconnected timeout, closing sub link",
					"user_id", snap.UserID,
					"disconnected_duration", now.Sub(snap.MainDisconnectedAt))
				g.store.CloseSubLink(snap.UserID)
			}
			// 主链路断开但还未超时，从链路保持
			continue
		}

		// 主链路正常的情况
		if snap.MainSessionID == "" {
			continue
		}

		// 检查从链路是否需要重连
		if !snap.SubConnected && snap.DownLinkIP != "" && snap.DownLinkPort > 0 {
			slog.Warn("sub link disconnected, triggering reconnect", "user_id", snap.UserID)
			go g.connectSubLinkWithRetry(snap.UserID, true)
		}
	}
	// 检查车辆定位状态
	g.checkVehiclePositions()
}

func (g *JT809Gateway) keepAliveSubLink(ctx context.Context, c *client.SimpleClient, userID uint32) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("sub heartbeat goroutine stopped", "user_id", userID)
			return
		case <-ticker.C:
			snap, ok := g.store.Snapshot(userID)
			if !ok {
				slog.Warn("skip sub heartbeat, snapshot missing", "user_id", userID)
				continue
			}
			if snap.GNSSCenterID == 0 {
				slog.Warn("skip sub heartbeat, missing GNSSCenterID", "user_id", userID)
				continue
			}
			hb, _ := jtt809.BuildSubLinkHeartbeat(jtt809.Header{
				GNSSCenterID: snap.GNSSCenterID,
			})
			g.logPacket("sub", "send", fmt.Sprintf("%d", userID), hb)
			if err := c.Send(hb); err != nil {
				slog.Warn("send sub heartbeat failed", "user_id", userID, "err", err)
				// 心跳失败，主动关闭连接以触发readSubLinkLoop退出
				c.Close()
				return
			}
		}
	}
}

func (g *JT809Gateway) handleHeartbeat(session *goserver.AppSession, frame *jtt809.Frame, isMain bool) ([]byte, error) {
	user, ok := g.sessionUser(session)
	if !ok {
		slog.Warn("heartbeat before login", "session", session.ID)
		return nil, nil
	}

	g.store.RecordHeartbeat(user, isMain)

	// 主链路心跳请求，应答通过从链路发送（支持降级到主链路）
	slog.Info("main link heartbeat", "session", session.ID, "user_id", user)
	resp := jtt809.HeartbeatResponse{}
	if err := g.sendResponseOnLink(true, user, frame, resp); err != nil {
		slog.Error("send heartbeat response failed", "user_id", user, "err", err)
	}
	return nil, nil
}

func (g *JT809Gateway) handleDynamicInfo(userID uint32, frame *jtt809.Frame) {
	pkt, err := jtt809.ParseSubBusiness(frame.RawBody)
	if err != nil {
		slog.Warn("parse sub business failed", "user_id", userID, "err", err)
		return
	}
	switch {
	case pkt.SubBusinessID == jtt809.SubMsgUploadVehicleReg:
		info, err := jtt809.ParseVehicleRegistration(pkt.Payload)
		if err != nil {
			slog.Warn("parse vehicle registration failed", "user_id", userID, "err", err)
			return
		}
		reg := &VehicleRegistration{
			PlatformID:        info.PlatformID,
			ProducerID:        info.ProducerID,
			TerminalModelType: info.TerminalModelType,
			IMEI:              info.IMEI,
			TerminalID:        info.TerminalID,
			TerminalSIM:       info.TerminalSIM,
		}
		g.store.UpdateVehicleRegistration(userID, pkt.Color, pkt.Plate, reg)
		slog.Info("vehicle registration", "user_id", userID, "plate", pkt.Plate, "platform", reg.PlatformID)

		// 触发车辆注册回调
		if g.callbacks != nil && g.callbacks.OnVehicleRegistration != nil {
			go g.callbacks.OnVehicleRegistration(userID, pkt.Plate, pkt.Color, reg)
		}

		// 自动订阅该车辆的实时定位数据
		go g.autoSubscribeVehicle(userID, pkt.Color, pkt.Plate)
	case pkt.SubBusinessID == jtt809.SubMsgRealLocation:
		pos, err := jtt809.ParseVehiclePosition(pkt.Payload)
		if err != nil {
			slog.Warn("parse vehicle position failed", "user_id", userID, "err", err)
			return
		}
		g.store.UpdateLocation(userID, pkt.Color, pkt.Plate, &pos, 0)

		// 触发车辆定位回调
		var gnssData *jtt809.GNSSData
		if gnss, err := jtt809.ParseGNSSData(pos.GnssData); err == nil {
			gnssData = &gnss
		}
		if g.callbacks != nil && g.callbacks.OnVehicleLocation != nil {
			go g.callbacks.OnVehicleLocation(userID, pkt.Plate, pkt.Color, &pos, gnssData)
		}
		if gnssData != nil {
			slog.Info("vehicle location", "user_id", userID, "plate", pkt.Plate, "lon", gnssData.Longitude, "lat", gnssData.Latitude)
		} else {
			slog.Info("vehicle location", "user_id", userID, "plate", pkt.Plate, "gnss_len", len(pos.GnssData))
		}
	case pkt.SubBusinessID == jtt809.SubMsgBatchLocation:
		if len(pkt.Payload) == 0 {
			return
		}
		count := int(pkt.Payload[0])
		reader := pkt.Payload[1:]
		parsed := 0
		for i := 0; i < count && len(reader) >= 5; i++ {
			gnssLen := int(binary.BigEndian.Uint32(reader[1:5]))
			totalLen := 1 + 4 + gnssLen + (11+4)*3
			if gnssLen < 0 || len(reader) < totalLen {
				break
			}
			pos, err := jtt809.ParseVehiclePosition(reader[:totalLen])
			if err != nil {
				slog.Warn("parse batch vehicle position failed", "user_id", userID, "index", i, "err", err)
				break
			}
			g.store.UpdateLocation(userID, pkt.Color, pkt.Plate, &pos, count)
			if gnss, err := jtt809.ParseGNSSData(pos.GnssData); err == nil {
				slog.Info("batch location item", "user_id", userID, "plate", pkt.Plate, "index", i, "lon", gnss.Longitude, "lat", gnss.Latitude)
			}
			reader = reader[totalLen:]
			parsed++
		}
		slog.Info("batch vehicle location", "user_id", userID, "plate", pkt.Plate, "count", parsed)

		// 触发批量定位回调
		if g.callbacks != nil && g.callbacks.OnBatchLocation != nil {
			go g.callbacks.OnBatchLocation(userID, pkt.Plate, pkt.Color, parsed)
		}
	case pkt.SubBusinessID == jtt809.SubMsgApplyForMonitorStartupAck:
		ack, err := jtt809.ParseMonitorAck(pkt.Payload)
		if err != nil {
			slog.Warn("parse monitor startup ack failed", "user_id", userID, "err", err, "payload_hex", fmt.Sprintf("%X", pkt.Payload))
			return
		}
		slog.Info("monitor startup ack received",
			"user_id", userID,
			"plate", pkt.Plate,
			"source_type", fmt.Sprintf("0x%04X", ack.SourceDataType),
			"source_sn", ack.SourceMsgSN,
			"data_length", ack.DataLength)

		// 触发启动车辆定位应答回调
		if g.callbacks != nil && g.callbacks.OnMonitorStartupAck != nil {
			go g.callbacks.OnMonitorStartupAck(userID, pkt.Plate, pkt.Color)
		}
	case pkt.SubBusinessID == jtt809.SubMsgApplyForMonitorEndAck:
		ack, err := jtt809.ParseMonitorAck(pkt.Payload)
		if err != nil {
			slog.Warn("parse monitor end ack failed", "user_id", userID, "err", err, "payload_hex", fmt.Sprintf("%X", pkt.Payload))
			return
		}
		// 收到应答表示下级平台已接收取消订阅请求
		slog.Info("monitor end ack received",
			"user_id", userID,
			"plate", pkt.Plate,
			"source_type", fmt.Sprintf("0x%04X", ack.SourceDataType),
			"source_sn", ack.SourceMsgSN)

		// 触发结束车辆定位应答回调
		if g.callbacks != nil && g.callbacks.OnMonitorEndAck != nil {
			go g.callbacks.OnMonitorEndAck(userID, pkt.Plate, pkt.Color)
		}
	default:
		slog.Debug("unhandled dynamic sub business", "user_id", userID, "sub_id", fmt.Sprintf("0x%04X", pkt.SubBusinessID))
	}
}

func (g *JT809Gateway) handlePlatformInfo(userID uint32, frame *jtt809.Frame) {
	pkt, err := jtt809.ParseSubBusiness(frame.RawBody)
	if err != nil {
		slog.Warn("parse platform info failed", "user_id", userID, "err", err)
		return
	}
	if pkt.SubBusinessID == jtt809.SubMsgPlatformQueryAck {
		ack, err := jtt809.ParsePlatformQueryAck(pkt)
		if err != nil {
			slog.Warn("parse platform query ack failed", "user_id", userID, "err", err)
			return
		}
		slog.Info("platform query ack", "user_id", userID, "object", ack.ObjectID, "info", ack.InfoContent)
		return
	}
	slog.Debug("unhandled platform info sub", "user_id", userID, "sub_id", fmt.Sprintf("0x%04X", pkt.SubBusinessID))
}





func (g *JT809Gateway) handleAlarmInteract(session *goserver.AppSession, frame *jtt809.Frame) {
	sessionID := ""
	if session != nil {
		sessionID = session.ID
	}
	slog.Debug("ignored alarm interact message", "session", sessionID, "msg_id", fmt.Sprintf("0x%04X", frame.BodyID))
}

func (g *JT809Gateway) handleDisconnectInform(session *goserver.AppSession, frame *jtt809.Frame) {
	disc, err := jtt809.ParseDisconnectInform(frame)
	if err != nil {
		slog.Warn("parse disconnect inform failed", "session", session.ID, "err", err)
		return
	}
	slog.Warn("platform disconnect notify", "session", session.ID, "code", disc.ErrorCode)
}

func (g *JT809Gateway) handleRealTimeVideo(userID uint32, frame *jtt809.Frame) {
	pkt, err := jtt809.ParseSubBusiness(frame.RawBody)
	if err != nil {
		slog.Warn("parse sub business failed", "user_id", userID, "err", err)
		return
	}
	if pkt.SubBusinessID == jtt809.SubMsgRealTimeVideoStartupAck {
		ack, err := jt1078.ParseRealTimeVideoStartupAck(pkt.Payload)
		if err != nil {
			slog.Warn("parse video ack failed", "user_id", userID, "err", err)
			return
		}
		g.store.RecordVideoAck(userID, pkt.Color, pkt.Plate, &VideoAckState{
			Result:     ack.Result,
			ServerIP:   ack.ServerIP,
			ServerPort: ack.ServerPort,
		})
		slog.Info("video stream ack", "user_id", userID, "plate", pkt.Plate, "server", ack.ServerIP, "port", ack.ServerPort, "result", ack.Result)

		// 触发视频应答回调
		if g.callbacks != nil && g.callbacks.OnVideoResponse != nil {
			go g.callbacks.OnVideoResponse(userID, pkt.Plate, pkt.Color, &VideoAckState{
				Result:     ack.Result,
				ServerIP:   ack.ServerIP,
				ServerPort: ack.ServerPort,
			})
		}
	}
}

func (g *JT809Gateway) handleSubDisconnect(userID uint32, frame *jtt809.Frame) {
	notify, err := jtt809.ParseDisconnectInform(frame)
	if err != nil {
		slog.Warn("parse main disconnect notify failed", "user_id", userID, "err", err)
		return
	}
	slog.Warn("main link disconnect notify", "user_id", userID, "code", notify.ErrorCode)
}

func isIPAllowed(ip string, allowIPs []string) bool {
	if len(allowIPs) == 0 {
		return true
	}
	for _, allow := range allowIPs {
		if allow == "*" {
			return true
		}
		if ip != "" && ip == allow {
			return true
		}
	}
	return false
}

func (g *JT809Gateway) getClientIP(session *goserver.AppSession) string {
	addr := session.RemoteAddr()
	host, _, err := net.SplitHostPort(addr.String())
	if err == nil {
		return host
	}
	return addr.String()
}

// shouldUseSameLink 判断响应是否应该使用与请求相同的链路
// 返回 true 表示使用相同链路（如登录消息）
// 返回 false 表示使用相反链路
func shouldUseSameLink(msgID uint16) bool {
	switch msgID {
	case jtt809.MsgIDLoginRequest, // 0x1001 主链路登录请求
		jtt809.MsgIDLoginResponse,    // 0x1002 主链路登录应答
		jtt809.MsgIDDownlinkConnReq,  // 0x9001 从链路登录请求
		jtt809.MsgIDDownlinkConnResp: // 从链路登录应答
		return true
	default:
		return false
	}
}

// selectResponseLink 根据接收链路和消息类型选择响应链路
// 返回值：("main"|"sub", shouldUseMain bool)
func selectResponseLink(receivedOnMain bool, msgID uint16) (linkName string, useMain bool) {
	// 登录消息使用相同链路
	if shouldUseSameLink(msgID) {
		if receivedOnMain {
			return "main", true
		}
		return "sub", false
	}

	// 其他消息使用相反链路
	if receivedOnMain {
		return "sub", false
	}
	return "main", true
}

// sendResponseOnLink 根据消息类型和链路状态选择合适的链路发送响应，支持降级
func (g *JT809Gateway) sendResponseOnLink(receivedOnMain bool, userID uint32, frame *jtt809.Frame, body jtt809.Body) error {
	pkg := jtt809.Package{
		Header: frame.Header.WithResponse(body.MsgID()),
		Body:   body,
	}
	data, err := jtt809.EncodePackage(pkg)
	if err != nil {
		return fmt.Errorf("encode package: %w", err)
	}

	// 选择响应链路
	linkName, useMain := selectResponseLink(receivedOnMain, frame.BodyID)
	mainActive, subActive := g.store.GetLinkStatus(userID)

	// 尝试首选链路
	if useMain {
		if mainActive {
			if sessionID, ok := g.store.GetMainSession(userID); ok {
				if session, err := g.mainSrv.GetSessionByID(sessionID); err == nil {
					g.logPacket(linkName, "send", session.ID, data)
					if err := session.Send(data); err == nil {
						return nil
					}
					slog.Warn("send on main link failed, try fallback", "user_id", userID, "err", err)
				}
			}
		}
		// 主链路不可用，降级到从链路
		if subActive {
			if subClient, ok := g.store.GetSubClient(userID); ok {
				slog.Info("main link unavailable, fallback to sub link", "user_id", userID, "msg_id", fmt.Sprintf("0x%04X", body.MsgID()))
				g.logPacket("sub(fallback)", "send", fmt.Sprintf("%d", userID), data)
				return subClient.Send(data)
			}
		}
	} else {
		// 首选从链路
		if subActive {
			if subClient, ok := g.store.GetSubClient(userID); ok {
				g.logPacket(linkName, "send", fmt.Sprintf("%d", userID), data)
				if err := subClient.Send(data); err == nil {
					return nil
				}
				slog.Warn("send on sub link failed, try fallback", "user_id", userID, "err", err)
			}
		}
		// 从链路不可用，降级到主链路
		if mainActive {
			if sessionID, ok := g.store.GetMainSession(userID); ok {
				if session, err := g.mainSrv.GetSessionByID(sessionID); err == nil {
					slog.Info("sub link unavailable, fallback to main link", "user_id", userID, "msg_id", fmt.Sprintf("0x%04X", body.MsgID()))
					g.logPacket("main(fallback)", "send", session.ID, data)
					return session.Send(data)
				}
			}
		}
	}

	return fmt.Errorf("no available link for platform %d", userID)
}

func (g *JT809Gateway) simpleResponse(session *goserver.AppSession, link string, frame *jtt809.Frame, body jtt809.Body) ([]byte, error) {
	pkg := jtt809.Package{
		Header: frame.Header.WithResponse(body.MsgID()),
		Body:   body,
	}
	data, err := jtt809.EncodePackage(pkg)
	if err == nil {
		g.logPacket(link, "send", session.ID, data)
	}
	return data, err
}

func (g *JT809Gateway) logPacket(link, dir, sessionID string, data []byte) {
	slog.Info("packet dump", "link", link, "dir", dir, "session", sessionID, "hex", fmt.Sprintf("%X", data))
}

func (g *JT809Gateway) sessionUser(session *goserver.AppSession) (uint32, bool) {
	val, err := session.GetAttr("userID")
	if err != nil {
		return 0, false
	}
	user, ok := val.(uint32)
	return user, ok
}

func (g *JT809Gateway) onSessionClosed(session *goserver.AppSession, reason string) {
	link, _ := session.GetAttr("link")
	slog.Info("session closed", "session", session.ID, "link", link, "reason", reason)
	g.store.RemoveSession(session.ID)
}

func splitJT809Frames(data []byte, atEOF bool) (advance int, token []byte, err error) {
	const (
		begin = byte(0x5b)
		end   = byte(0x5d)
	)
	start := bytes.IndexByte(data, begin)
	if start == -1 {
		if atEOF {
			return len(data), nil, nil
		}
		return 0, nil, nil
	}
	if start > 0 {
		return start, nil, nil
	}
	stop := bytes.IndexByte(data[1:], end)
	if stop == -1 {
		if atEOF {
			return len(data), nil, fmt.Errorf("dangling frame")
		}
		return 0, nil, nil
	}
	stop++ // compensate slicing offset
	frame := make([]byte, stop+1)
	copy(frame, data[:stop+1])
	return stop + 1, frame, nil
}

func (g *JT809Gateway) handleAuthorize(userID uint32, frame *jtt809.Frame) {
	// 0x1700 消息体结构：子业务ID(2字节) + 载荷
	// 注意：不包含车牌号和颜色，与 0x1200 的 SubBusinessPacket 格式不同
	msg, err := jt1078.ParseAuthorizeMsg(frame.RawBody)
	if err != nil {
		slog.Warn("parse authorize msg failed", "user_id", userID, "err", err)
		return
	}
	switch msg.SubBusinessID {
	case jtt809.SubMsgAuthorizeStartupReq: // 0x1701
		req, err := jt1078.ParseAuthorizeStartupReq(msg.Payload)
		if err != nil {
			slog.Warn("parse authorize startup req failed", "user_id", userID, "err", err)
			return
		}
		authCode := req.AuthorizeCode1
		// 注意：0x1700 消息中没有车牌号和颜色信息，时效口令是平台级别的
		g.store.UpdateAuthCode(userID, req.PlatformID, authCode)
		slog.Info("video authorize report", "user_id", userID, "platform", req.PlatformID, "auth_code", authCode)

		// 触发鉴权回调
		if g.callbacks != nil && g.callbacks.OnAuthorize != nil {
			go g.callbacks.OnAuthorize(userID, req.PlatformID, authCode)
		}

	default:
		slog.Debug("unhandled authorize sub msg", "user_id", userID, "sub_id", fmt.Sprintf("0x%04X", msg.SubBusinessID))
	}
}

// autoSubscribeVehicle 在车辆注册后自动订阅该车辆的实时定位数据
func (g *JT809Gateway) autoSubscribeVehicle(userID uint32, color jtt809.PlateColor, vehicle string) {
	// 等待一小段时间，确保从链路已建立
	time.Sleep(2 * time.Second)

	req := MonitorRequest{
		UserID:       userID,
		VehicleNo:    vehicle,
		VehicleColor: color,
		ReasonCode:   byte(jtt809.MonitorReasonManual),
	}

	if err := g.RequestMonitorStartup(req); err != nil {
		slog.Warn("auto subscribe vehicle failed", "user_id", userID, "plate", vehicle, "err", err)
		return
	}

	slog.Info("auto subscribed vehicle", "user_id", userID, "plate", vehicle)
}

// checkVehiclePositions 检查所有车辆的定位状态，处理超时和离线车辆
func (g *JT809Gateway) checkVehiclePositions() {
	snapshots := g.store.Snapshots()
	now := time.Now()

	for _, snap := range snapshots {
		if snap.MainSessionID == "" || !snap.SubConnected {
			continue
		}

		for _, vehicle := range snap.Vehicles {
			vehicleKey := vehicleKey(vehicle.VehicleNo, vehicle.VehicleColor)

			// 处理已注册但还未订阅或订阅失败的车辆
			if vehicle.PositionTime.IsZero() {
				// 如果有注册信息，说明车辆已注册，尝试订阅
				if vehicle.Registration != nil {
					// 先判断注册时间是否超过10分钟，超过则删除
					if now.Sub(vehicle.Registration.ReceivedAt) > 10*time.Minute {
						g.store.RemoveVehicle(snap.UserID, vehicleKey)
						slog.Warn("vehicle registration expired, removed",
							"user_id", snap.UserID,
							"plate", vehicle.VehicleNo,
							"registration_time", vehicle.Registration.ReceivedAt.Format("2006-01-02 15:04:05"))
						continue
					}

					req := MonitorRequest{
						UserID:       snap.UserID,
						VehicleNo:    vehicle.VehicleNo,
						VehicleColor: vehicle.VehicleColor,
						ReasonCode:   byte(jtt809.MonitorReasonManual),
					}

					if err := g.RequestMonitorStartup(req); err != nil {
						slog.Warn("subscribe registered vehicle failed",
							"user_id", snap.UserID,
							"plate", vehicle.VehicleNo,
							"err", err)
						continue
					}

					slog.Info("subscribed registered vehicle",
						"user_id", snap.UserID,
						"plate", vehicle.VehicleNo)
				}
				continue
			}

			timeSinceLastPosition := now.Sub(vehicle.PositionTime)

			// 超过10分钟未上报定位，认定为离线，删除车辆
			if timeSinceLastPosition > 10*time.Minute {
				g.store.RemoveVehicle(snap.UserID, vehicleKey)
				slog.Warn("vehicle offline, removed",
					"user_id", snap.UserID,
					"plate", vehicle.VehicleNo,
					"last_position", vehicle.PositionTime.Format("2006-01-02 15:04:05"),
					"offline_duration", timeSinceLastPosition.String())
				continue
			}

			// 超过5分钟未上报定位，重新发送订阅请求
			if timeSinceLastPosition > 5*time.Minute {
				req := MonitorRequest{
					UserID:       snap.UserID,
					VehicleNo:    vehicle.VehicleNo,
					VehicleColor: vehicle.VehicleColor,
					ReasonCode:   byte(jtt809.MonitorReasonManual),
				}

				if err := g.RequestMonitorStartup(req); err != nil {
					slog.Warn("resubscribe vehicle failed",
						"user_id", snap.UserID,
						"plate", vehicle.VehicleNo,
						"err", err)
					continue
				}

				slog.Info("resubscribed vehicle due to position timeout",
					"user_id", snap.UserID,
					"plate", vehicle.VehicleNo,
					"last_position", vehicle.PositionTime.Format("2006-01-02 15:04:05"),
					"timeout_duration", timeSinceLastPosition.String())
			}
		}
	}
}

// SendDownlinkMessage 发送下行消息，支持主/从链路自动选择与降级
// 优先使用从链路，不可用时降级到主链路
func (g *JT809Gateway) SendDownlinkMessage(userID uint32, body jtt809.Body) error {
	snap, ok := g.store.Snapshot(userID)
	if !ok || snap.MainSessionID == "" {
		return fmt.Errorf("platform %d not online", userID)
	}

	header := jtt809.Header{
		GNSSCenterID: snap.GNSSCenterID,
		Version:      jtt809.Version{Major: 1, Minor: 0, Patch: 0},
		EncryptFlag:  0,
		EncryptKey:   0,
		BusinessType: body.MsgID(),
	}
	pkg := jtt809.Package{
		Header: header,
		Body:   body,
	}
	data, err := jtt809.EncodePackage(pkg)
	if err != nil {
		return fmt.Errorf("encode package: %w", err)
	}

	_, subActive := g.store.GetLinkStatus(userID)

	// 优先尝试从链路
	if subActive {
		g.store.mu.RLock()
		subClient := g.store.platforms[userID].SubClient
		g.store.mu.RUnlock()
		if subClient != nil {
			g.logPacket("sub", "send", fmt.Sprintf("%d", userID), data)
			if err := subClient.Send(data); err == nil {
				return nil
			}
			slog.Warn("send on sub link failed, try fallback", "user_id", userID, "err", err)
		}
	}

	// 降级到主链路
	if sessionID := snap.MainSessionID; sessionID != "" {
		if session, err := g.mainSrv.GetSessionByID(sessionID); err == nil {
			slog.Info("sub link unavailable, fallback to main link", "user_id", userID, "msg_id", fmt.Sprintf("0x%04X", body.MsgID()))
			g.logPacket("main(fallback)", "send", session.ID, data)
			return session.Send(data)
		}
	}

	return fmt.Errorf("no available link for platform %d", userID)
}
