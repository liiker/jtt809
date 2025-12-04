package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ================= 常量定义 =================
var (
	port        = ":8080"
	magicHeader = []byte{0x30, 0x31, 0x63, 0x64}
	startCode   = []byte{0x00, 0x00, 0x00, 0x01}
)

// ================= 核心结构体 =================

type StreamManager struct {
	streams sync.Map
}

type Broadcaster struct {
	url     string
	clients map[chan []byte]string // 值存储 IP，用于日志
	lock    sync.RWMutex
	running bool

	// GOP Cache
	gopCache [][]byte
	gopLock  sync.RWMutex

	frameAssemblyBuffer *bytes.Buffer
}

var manager = &StreamManager{}

// ================= 主程序 =================

func main() {
	// 开启详细日志：日期 时间 微秒
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	http.HandleFunc("/proxy", handleProxyRaw)
	http.HandleFunc("/proxy.flv", handleProxyFLV)

	fmt.Println("===================================================")
	fmt.Println("🚀 JT/T 1078-2016 最终完美版 (Logs + Fixes)")
	fmt.Println("✨ 功能: 视频秒开 | 多路复用 | 延迟自动修复 | 全链路日志")
	fmt.Printf("👂 监听端口: %s\n", port)
	fmt.Println("===================================================")

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

// ================= HTTP 处理逻辑 =================

func handleProxyRaw(w http.ResponseWriter, r *http.Request) {
	targetURL, clientIP := parseRequest(r)
	if targetURL == "" {
		http.Error(w, "missing url", 400)
		return
	}

	w.Header().Set("Content-Type", "video/x-h264")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	runStreamLoop(w, flusher, targetURL, clientIP, false)
}

func handleProxyFLV(w http.ResponseWriter, r *http.Request) {
	targetURL, clientIP := parseRequest(r)
	if targetURL == "" {
		http.Error(w, "missing url", 400)
		return
	}

	w.Header().Set("Content-Type", "video/x-flv")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	// 发送 FLV Header
	w.Write([]byte{'F', 'L', 'V', 0x01, 0x01, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x00})
	runStreamLoop(w, flusher, targetURL, clientIP, true)
}

func runStreamLoop(w http.ResponseWriter, flusher http.Flusher, targetURL, clientIP string, isFLV bool) {
	broadcaster := manager.GetOrCreateBroadcaster(targetURL)

	clientChan := make(chan []byte, 1000)

	// 订阅 (内部打印日志)
	cachedGOP := broadcaster.Subscribe(clientChan, clientIP)
	defer broadcaster.Unsubscribe(clientChan)

	var muxer *FlvMuxer
	if isFLV {
		muxer = NewFlvMuxer()
	}

	processFrame := func(frame []byte) error {
		if isFLV {
			tags, err := muxer.WriteFrame(frame)
			if err != nil {
				return nil
			}
			for _, tag := range tags {
				if _, err := w.Write(tag); err != nil {
					return err
				}
			}
		} else {
			if _, err := w.Write(frame); err != nil {
				return err
			}
		}
		return nil
	}

	// 1. 发送缓存 (秒开)
	for _, frame := range cachedGOP {
		if err := processFrame(frame); err != nil {
			return
		}
	}
	flusher.Flush()

	// 2. 实时转发
	for {
		frameData, isOpen := <-clientChan
		if !isOpen {
			return
		}
		if err := processFrame(frameData); err != nil {
			return
		}
		flusher.Flush()
	}
}

// ================= 广播器逻辑 (日志 + 缓存修复) =================

func (m *StreamManager) GetOrCreateBroadcaster(targetURL string) *Broadcaster {
	if val, ok := m.streams.Load(targetURL); ok {
		return val.(*Broadcaster)
	}

	newB := &Broadcaster{
		url:                 targetURL,
		clients:             make(map[chan []byte]string),
		running:             true,
		gopCache:            make([][]byte, 0, 500),
		frameAssemblyBuffer: bytes.NewBuffer(make([]byte, 0, 512*1024)),
	}
	actual, loaded := m.streams.LoadOrStore(targetURL, newB)
	b := actual.(*Broadcaster)
	if !loaded {
		// 日志: 新流启动
		log.Printf("✨ [New Stream] 启动拉流任务: %s", shortenURL(targetURL))
		go b.StartPulling()
	}
	return b
}

func (b *Broadcaster) Subscribe(ch chan []byte, clientIP string) [][]byte {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.clients[ch] = clientIP

	// 日志: 客户端加入
	log.Printf("➕ [Client Join] IP: %s | 在线: %d | 流: ...%s",
		clientIP, len(b.clients), shortenURL(b.url))

	b.gopLock.RLock()
	defer b.gopLock.RUnlock()
	snapshot := make([][]byte, len(b.gopCache))
	copy(snapshot, b.gopCache)
	return snapshot
}

func (b *Broadcaster) Unsubscribe(ch chan []byte) {
	b.lock.Lock()
	defer b.lock.Unlock()
	ip := b.clients[ch]
	delete(b.clients, ch)

	// 日志: 客户端离开
	log.Printf("➖ [Client Left] IP: %s | 在线: %d | 流: ...%s",
		ip, len(b.clients), shortenURL(b.url))

	if len(b.clients) == 0 {
		log.Printf("🗑️ [Stream Stop] 无人观看，销毁流任务: ...%s", shortenURL(b.url))
		manager.streams.Delete(b.url)
		b.running = false
	}
}

func (b *Broadcaster) updateGOPCache(frame []byte, isKeyFrame bool) {
	b.gopLock.Lock()
	defer b.gopLock.Unlock()

	if isKeyFrame {
		b.gopCache = b.gopCache[:0]
	}

	// 【重要修复】防止缓存无限增长导致 Web 端延迟过大
	if len(b.gopCache) > 500 {
		b.gopCache = b.gopCache[:0]
	}

	b.gopCache = append(b.gopCache, frame)
}

func (b *Broadcaster) broadcast(frame []byte) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- frame:
		default:
		}
	}
}

func (b *Broadcaster) StartPulling() {
	log.Printf("🔗 [Source Connect] 开始连接上级平台...")

	client := &http.Client{Timeout: 0}
	req, _ := http.NewRequest("GET", b.url, nil)
	req.Header.Set("User-Agent", "JT1078-Proxy/LogVersion") // 加上 UA 防止被拒
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ [Source Error] 连接失败: %v", err)
		manager.streams.Delete(b.url)
		return
	}
	defer resp.Body.Close()

	log.Printf("✅ [Source OK] 连接成功，开始拉流")

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 2<<20)
	scanner.Buffer(buf, 5<<20)
	scanner.Split(func(d []byte, atEOF bool) (int, []byte, error) {
		if atEOF && len(d) == 0 {
			return 0, nil, nil
		}
		i := bytes.Index(d, magicHeader)
		if i < 0 {
			if atEOF {
				return len(d), nil, nil
			}
			return 0, nil, nil
		}
		if i > 0 {
			return i, nil, nil
		}
		if len(d) < 16 {
			return 0, nil, nil
		}
		hLen := 30
		dt := d[15] >> 4
		if dt == 3 {
			hLen = 26
		} else if dt == 4 {
			hLen = 18
		}
		if len(d) < hLen {
			return 0, nil, nil
		}
		pLen := hLen + int(binary.BigEndian.Uint16(d[hLen-2:hLen]))
		if len(d) < pLen {
			return 0, nil, nil
		}
		return pLen, d[:pLen], nil
	})

	lastLogTime := time.Now()
	totalBytes := 0

	for b.running && scanner.Scan() {
		packet := scanner.Bytes()
		totalBytes += len(packet)

		// 日志: 心跳保活，每30秒打印流量
		if time.Since(lastLogTime) > 30*time.Second {
			log.Printf("💓 [KeepAlive] 流 ...%s 正常 | 30秒流量: %.2f MB",
				shortenURL(b.url), float64(totalBytes)/1024/1024)
			lastLogTime = time.Now()
			totalBytes = 0
		}

		b.processPacket(packet)
	}

	log.Printf("🛑 [Source Disconnect] 源断开: ...%s", shortenURL(b.url))
	manager.streams.Delete(b.url)
}

func (b *Broadcaster) processPacket(packet []byte) {
	if len(packet) < 16 {
		return
	}
	tag := packet[15] & 0x0F
	dt := packet[15] >> 4
	hLen := 30
	if dt == 3 {
		hLen = 26
	} else if dt == 4 {
		hLen = 18
	}
	if len(packet) < hLen {
		return
	}
	body := packet[hLen:]

	if dt <= 2 {
		if tag == 0 || tag == 1 {
			b.frameAssemblyBuffer.Write(startCode)
		}
		b.frameAssemblyBuffer.Write(body)
		if tag == 0 || tag == 2 {
			fullFrame := make([]byte, b.frameAssemblyBuffer.Len())
			copy(fullFrame, b.frameAssemblyBuffer.Bytes())

			isKey := (dt == 0)
			b.updateGOPCache(fullFrame, isKey)
			b.broadcast(fullFrame)
			b.frameAssemblyBuffer.Reset()
		}
	}
}

// ================= FLV 封装器 (智能时钟版) =================

type FlvMuxer struct {
	pps, sps       []byte
	sentConf       bool
	timestamp      uint32    // 当前 FLV 时间戳
	lastSystemTime time.Time // 上一次发送数据的物理时间
}

func NewFlvMuxer() *FlvMuxer {
	return &FlvMuxer{
		timestamp:      0,
		lastSystemTime: time.Time{}, // 零值初始化
	}
}

func (m *FlvMuxer) WriteFrame(frame []byte) ([][]byte, error) {
	nalus := bytes.Split(frame, startCode)
	var tags [][]byte

	// --- 核心修复逻辑开始 ---
	now := time.Now()

	// 如果是第一帧
	if m.lastSystemTime.IsZero() {
		m.lastSystemTime = now
	}

	// 计算距离上一帧的物理时间差 (毫秒)
	delta := uint32(now.Sub(m.lastSystemTime).Milliseconds())

	// 策略判断：
	// 1. 如果 delta 非常小 (< 10ms)，说明正在全速发送 GOP 缓存 (Burst 模式)
	//    此时强制按 30fps (33ms) 递增，帮客户端快速建立缓冲区。
	// 2. 如果 delta 正常 (> 10ms)，说明是实时流 (Live 模式)
	//    此时按真实流逝的时间递增，完美匹配上游的网络节奏。

	increment := delta
	if increment < 10 {
		increment = 33 // 强制 33ms (约30帧/秒)
	}

	// 防止时间戳跳变过大 (比如上游断了10秒后重连)，限制最大间隔，防止播放器跳进度条
	// 但对于监控流，真实反映卡顿可能比跳帧更好，这里暂时不做硬性上限，
	// 或者限制最大为 500ms (两帧之间最大停顿半秒，再久就认为丢帧了)
	/*
		if increment > 1000 {
			increment = 33 // 异常跳变回落
		}
	*/

	m.timestamp += increment
	m.lastSystemTime = now // 更新最后发送时间

	ts := m.timestamp
	// --- 核心修复逻辑结束 ---

	var vp bytes.Buffer
	isKey := false

	for _, nal := range nalus {
		if len(nal) == 0 {
			continue
		}
		t := nal[0] & 0x1F
		if t == 7 {
			m.sps = make([]byte, len(nal))
			copy(m.sps, nal)
		}
		if t == 8 {
			m.pps = make([]byte, len(nal))
			copy(m.pps, nal)
		}
		if t == 5 {
			isKey = true
		}
		binary.Write(&vp, binary.BigEndian, uint32(len(nal)))
		vp.Write(nal)
	}

	if len(m.sps) > 0 && len(m.pps) > 0 && !m.sentConf {
		tags = append(tags, m.createSeqHeader())
		m.sentConf = true
	}

	if vp.Len() > 0 {
		f := byte(0x27)
		if isKey {
			f = 0x17
		}
		d := new(bytes.Buffer)
		d.WriteByte(f)
		d.WriteByte(0x01)
		d.Write([]byte{0, 0, 0})
		d.Write(vp.Bytes())
		tags = append(tags, createFLVTag(9, d.Bytes(), ts))
	}
	return tags, nil
}

func (m *FlvMuxer) createSeqHeader() []byte {
	d := new(bytes.Buffer)
	d.WriteByte(0x17)
	d.WriteByte(0x00)
	d.Write([]byte{0, 0, 0})
	d.WriteByte(0x01)
	d.WriteByte(m.sps[1])
	d.WriteByte(m.sps[2])
	d.WriteByte(m.sps[3])
	d.WriteByte(0xFF)
	d.WriteByte(0xE1)
	binary.Write(d, binary.BigEndian, uint16(len(m.sps)))
	d.Write(m.sps)
	d.WriteByte(0x01)
	binary.Write(d, binary.BigEndian, uint16(len(m.pps)))
	d.Write(m.pps)
	return createFLVTag(9, d.Bytes(), 0)
}

func createFLVTag(t byte, d []byte, ts uint32) []byte {
	sz := len(d)
	tot := 11 + sz + 4
	buf := make([]byte, tot)
	buf[0] = t
	buf[1] = byte(sz >> 16)
	buf[2] = byte(sz >> 8)
	buf[3] = byte(sz)
	buf[4] = byte(ts >> 16)
	buf[5] = byte(ts >> 8)
	buf[6] = byte(ts)
	buf[7] = byte(ts >> 24)
	copy(buf[11:], d)
	binary.BigEndian.PutUint32(buf[tot-4:], uint32(tot-4))
	return buf
}

// ================= 辅助函数 =================

func parseRequest(r *http.Request) (string, string) {
	u := r.URL.Query().Get("url")
	if decoded, err := url.QueryUnescape(u); err == nil && strings.HasPrefix(decoded, "http") {
		u = decoded
	}
	return u, r.RemoteAddr
}

func shortenURL(u string) string {
	if len(u) > 50 {
		return u[len(u)-50:]
	}
	return u
}
