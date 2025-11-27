package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/zboyco/jtt809/pkg/jtt809"
)

var (
	mainAddr     = flag.String("main", "127.0.0.1:10709", "Main Link Address")
	subPort      = flag.Int("sub", 9000, "Sub Link Listen Port")
	userID       = flag.Int("uid", 10001, "User ID")
	password     = flag.String("pwd", "pass809", "Password")
	myIP         = flag.String("ip", "127.0.0.1", "My IP for Sub Link")
	vehicleNo    = flag.String("vehicle", "粤B12345", "Vehicle Plate Number")
	vehicleColor = flag.Int("color", 2, "Vehicle Plate Color (1=Black, 2=Blue, 3=Yellow, 4=White, 9=Other)")
	locationSec  = flag.Int("location", 10, "GPS Location Report Interval in Seconds (0=disabled)")
)

func main() {
	flag.Parse()

	// 1. Start Sub Link Listener
	subListener, err := net.Listen("tcp", fmt.Sprintf(":%d", *subPort))
	if err != nil {
		log.Fatalf("Failed to listen on sub port %d: %v", *subPort, err)
	}
	defer subListener.Close()
	log.Printf("Sub Link listening on %s:%d", *myIP, *subPort)

	go acceptSubLink(subListener)

	// 2. Connect to Main Link
	log.Printf("Connecting to Main Link %s...", *mainAddr)
	conn, err := net.Dial("tcp", *mainAddr)
	if err != nil {
		log.Fatalf("Failed to connect to main link: %v", err)
	}
	defer conn.Close()
	log.Println("Connected to Main Link")

	// 3. Send Login Request
	pkg, err := jtt809.BuildLoginPackage(jtt809.Header{
		MsgSN:   1,
		Version: jtt809.Version{Major: 1, Minor: 0, Patch: 0},
	}, jtt809.LoginRequest{
		UserID:          uint32(*userID),
		Password:        *password,
		DownLinkIP:      *myIP,
		DownLinkPort:    uint16(*subPort),
		ProtocolVersion: [3]byte{1, 2, 19}, // 2019版本
	})
	if err != nil {
		log.Fatalf("Build login failed: %v", err)
	}

	log.Printf("Sending Login Request: %X", pkg)
	if _, err := conn.Write(pkg); err != nil {
		log.Fatalf("Send login failed: %v", err)
	}

	// Start heartbeat goroutine after login
	msgSN := uint32(2) // Start from 2, 1 was used for login
	go sendMainHeartbeat(conn, &msgSN)

	// 4. Read Loop
	scanner := bufio.NewScanner(conn)
	scanner.Split(splitJT809Frames)
	loginSuccess := false
	for scanner.Scan() {
		data := scanner.Bytes()
		log.Printf("[Main] Received: %X", data)
		frame, err := jtt809.DecodeFrame(data)
		if err != nil {
			log.Printf("Decode failed: %v", err)
			continue
		}
		switch frame.BodyID {
		case jtt809.MsgIDLoginResponse:
			log.Println("Login Response Received")
			if !loginSuccess {
				loginSuccess = true
				// Send vehicle registration
				go sendVehicleRegistration(conn, &msgSN)
				// Start GPS location updates
				if *locationSec > 0 {
					go sendLocationUpdates(conn, &msgSN, time.Duration(*locationSec)*time.Second)
				}
			}
		case jtt809.MsgIDHeartbeatResponse:
			log.Println("Heartbeat Response Received")
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Main link read error: %v", err)
	}
}

func sendMainHeartbeat(conn net.Conn, msgSN *uint32) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		hb, err := jtt809.EncodePackage(jtt809.Package{
			Header: jtt809.Header{
				MsgSN:        *msgSN,
				BusinessType: jtt809.MsgIDHeartbeatRequest,
				Version:      jtt809.Version{Major: 1, Minor: 0, Patch: 0},
			},
			Body: jtt809.HeartbeatRequest{},
		})
		if err != nil {
			log.Printf("Build heartbeat failed: %v", err)
			continue
		}
		*msgSN++

		log.Printf("[Main] Sending Heartbeat: %X", hb)
		if _, err := conn.Write(hb); err != nil {
			log.Printf("Send heartbeat failed: %v", err)
			return
		}
	}
}

func acceptSubLink(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("Accept failed: %v", err)
			continue
		}
		log.Printf("Sub Link Incoming Connection from %s", conn.RemoteAddr())
		go handleSubLink(conn)
	}
}

func handleSubLink(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Split(splitJT809Frames)

	for scanner.Scan() {
		data := scanner.Bytes()
		log.Printf("[Sub] Received: %X", data)
		frame, err := jtt809.DecodeFrame(data)
		if err != nil {
			log.Printf("Sub decode failed: %v", err)
			continue
		}
		switch frame.BodyID {
		case jtt809.MsgIDDownlinkConnReq:
			log.Println("Sub Link Login Request Received")
			// Send Response
			resp := jtt809.SubLinkLoginResponse{Result: 0}
			pkg, _ := jtt809.EncodePackage(jtt809.Package{
				Header: frame.Header.WithResponse(0x9002),
				Body:   resp,
			})
			conn.Write(pkg)
			log.Printf("[Sub] Sent Login Response")
		case 0x9005: // Heartbeat
			log.Println("Sub Link Heartbeat Received")
			// Send Response
			pkg, _ := jtt809.EncodePackage(jtt809.Package{
				Header: frame.Header.WithResponse(0x9006),
				Body:   jtt809.SubLinkHeartbeatResponse{},
			})
			conn.Write(pkg)
		}
	}
}

func splitJT809Frames(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Simple split function copy-pasted or simplified
	// For simulator, we can use a simplified version or just copy the one from gateway
	const (
		begin = byte(0x5b)
		end   = byte(0x5d)
	)
	// ... (implementation similar to gateway.go)
	// For brevity in this tool call, I'll implement a simple one here
	if len(data) == 0 {
		return 0, nil, nil
	}
	start := -1
	for i, b := range data {
		if b == begin {
			start = i
			break
		}
	}
	if start == -1 {
		return len(data), nil, nil // discard junk
	}
	if start > 0 {
		return start, nil, nil // discard junk before start
	}
	// start is 0
	endIdx := -1
	for i := 1; i < len(data); i++ {
		if data[i] == end {
			endIdx = i
			break
		}
	}
	if endIdx == -1 {
		if atEOF {
			return len(data), nil, fmt.Errorf("incomplete frame")
		}
		return 0, nil, nil
	}
	return endIdx + 1, data[:endIdx+1], nil
}

// GPSSimulator 模拟GPS数据生成器
type GPSSimulator struct {
	lon       float64 // 当前经度
	lat       float64 // 当前纬度
	direction uint16  // 当前方向 0-359
	speed     uint16  // 当前速度 km/h
	mileage   uint32  // 总里程 km
}

// NewGPSSimulator 创建GPS模拟器，初始位置在深圳市中心
func NewGPSSimulator() *GPSSimulator {
	return &GPSSimulator{
		lon:       114.057868, // 深圳市中心经度
		lat:       22.543099,  // 深圳市中心纬度
		direction: 90,         // 初始方向为东
		speed:     40,         // 初始速度40km/h
		mileage:   0,
	}
}

// Next 生成下一个GPS位置数据
func (g *GPSSimulator) Next() jtt809.VehiclePosition {
	// 模拟运动：根据速度和方向更新位置
	// 简化计算：每次移动约0.0001度（约10米）
	deltaLon := float64(g.speed) * 0.0001 * float64(cosTable[g.direction%360]) / 100
	deltaLat := float64(g.speed) * 0.0001 * float64(sinTable[g.direction%360]) / 100

	g.lon += deltaLon
	g.lat += deltaLat

	// 随机改变速度和方向（模拟真实行驶）
	if time.Now().Unix()%5 == 0 {
		g.speed = uint16(20 + time.Now().Unix()%40) // 20-60 km/h
		g.direction = uint16(time.Now().Unix() % 360)
	}

	g.mileage += uint32(g.speed) / 3600 // 粗略计算里程

	return jtt809.VehiclePosition{
		Encrypt:     0,
		Time:        time.Now(),
		Lon:         uint32(g.lon * 1000000), // 转换为1e-6度
		Lat:         uint32(g.lat * 1000000),
		Speed:       g.speed,
		RecordSpeed: g.speed,
		Mileage:     g.mileage,
		Direction:   g.direction,
		Altitude:    50, // 海拔50米
		State:       0,  // 车辆状态
		Alarm:       0,  // 无报警
	}
}

// 简化的三角函数表（cos和sin的整数近似值，乘以100）
var cosTable = [360]int16{
	100, 100, 100, 99, 99, 98, 98, 97, 95, 94, 93, 91, 89, 87, 85, 82, 80, 77, 74, 71,
	68, 64, 61, 57, 54, 50, 46, 42, 38, 34, 30, 26, 22, 17, 13, 9, 5, 0, -4, -9,
	-13, -17, -22, -26, -30, -34, -38, -42, -46, -50, -54, -57, -61, -64, -68, -71, -74, -77, -80, -82,
	-85, -87, -89, -91, -93, -94, -95, -97, -98, -98, -99, -99, -100, -100, -100, -100, -99, -99, -98, -98,
	-97, -95, -94, -93, -91, -89, -87, -85, -82, -80, -77, -74, -71, -68, -64, -61, -57, -54, -50, -46,
	-42, -38, -34, -30, -26, -22, -17, -13, -9, -5, 0, 4, 9, 13, 17, 22, 26, 30, 34, 38,
	42, 46, 50, 54, 57, 61, 64, 68, 71, 74, 77, 80, 82, 85, 87, 89, 91, 93, 94, 95,
	97, 98, 98, 99, 99, 100, 100, 100, 100, 99, 99, 98, 98, 97, 95, 94, 93, 91, 89, 87,
	85, 82, 80, 77, 74, 71, 68, 64, 61, 57, 54, 50, 46, 42, 38, 34, 30, 26, 22, 17,
	13, 9, 5, 0, -4, -9, -13, -17, -22, -26, -30, -34, -38, -42, -46, -50, -54, -57, -61, -64,
	-68, -71, -74, -77, -80, -82, -85, -87, -89, -91, -93, -94, -95, -97, -98, -98, -99, -99, -100, -100,
	-100, -100, -99, -99, -98, -98, -97, -95, -94, -93, -91, -89, -87, -85, -82, -80, -77, -74, -71, -68,
	-64, -61, -57, -54, -50, -46, -42, -38, -34, -30, -26, -22, -17, -13, -9, -5, 0, 4, 9, 13,
	17, 22, 26, 30, 34, 38, 42, 46, 50, 54, 57, 61, 64, 68, 71, 74, 77, 80, 82, 85,
	87, 89, 91, 93, 94, 95, 97, 98, 98, 99, 99, 100, 100, 100, 100, 99, 99, 98, 98, 97,
	95, 94, 93, 91, 89, 87, 85, 82, 80, 77, 74, 71, 68, 64, 61, 57, 54, 50, 46, 42,
	38, 34, 30, 26, 22, 17, 13, 9, 5, 0, -4, -9, -13, -17, -22, -26, -30, -34, -38, -42,
	-46, -50, -54, -57, -61, -64, -68, -71, -74, -77, -80, -82, -85, -87, -89, -91, -93, -94, -95, -97,
}

var sinTable = [360]int16{
	0, 5, 9, 13, 17, 22, 26, 30, 34, 38, 42, 46, 50, 54, 57, 61, 64, 68, 71, 74,
	77, 80, 82, 85, 87, 89, 91, 93, 94, 95, 97, 98, 98, 99, 99, 100, 100, 100, 100, 99,
	99, 98, 98, 97, 95, 94, 93, 91, 89, 87, 85, 82, 80, 77, 74, 71, 68, 64, 61, 57,
	54, 50, 46, 42, 38, 34, 30, 26, 22, 17, 13, 9, 5, 0, -4, -9, -13, -17, -22, -26,
	-30, -34, -38, -42, -46, -50, -54, -57, -61, -64, -68, -71, -74, -77, -80, -82, -85, -87, -89, -91,
	-93, -94, -95, -97, -98, -98, -99, -99, -100, -100, -100, -100, -99, -99, -98, -98, -97, -95, -94, -93,
	-91, -89, -87, -85, -82, -80, -77, -74, -71, -68, -64, -61, -57, -54, -50, -46, -42, -38, -34, -30,
	-26, -22, -17, -13, -9, -5, 0, 4, 9, 13, 17, 22, 26, 30, 34, 38, 42, 46, 50, 54,
	57, 61, 64, 68, 71, 74, 77, 80, 82, 85, 87, 89, 91, 93, 94, 95, 97, 98, 98, 99,
	99, 100, 100, 100, 100, 99, 99, 98, 98, 97, 95, 94, 93, 91, 89, 87, 85, 82, 80, 77,
	74, 71, 68, 64, 61, 57, 54, 50, 46, 42, 38, 34, 30, 26, 22, 17, 13, 9, 5, 0,
	-4, -9, -13, -17, -22, -26, -30, -34, -38, -42, -46, -50, -54, -57, -61, -64, -68, -71, -74, -77,
	-80, -82, -85, -87, -89, -91, -93, -94, -95, -97, -98, -98, -99, -99, -100, -100, -100, -100, -99, -99,
	-98, -98, -97, -95, -94, -93, -91, -89, -87, -85, -82, -80, -77, -74, -71, -68, -64, -61, -57, -54,
	-50, -46, -42, -38, -34, -30, -26, -22, -17, -13, -9, -5, 0, 4, 9, 13, 17, 22, 26, 30,
	34, 38, 42, 46, 50, 54, 57, 61, 64, 68, 71, 74, 77, 80, 82, 85, 87, 89, 91, 93,
	94, 95, 97, 98, 98, 99, 99, 100, 100, 100, 100, 99, 99, 98, 98, 97, 95, 94, 93, 91,
	89, 87, 85, 82, 80, 77, 74, 71, 68, 64, 61, 57, 54, 50, 46, 42, 38, 34, 30, 26,
}

// sendVehicleRegistration 发送车辆注册信息
func sendVehicleRegistration(conn net.Conn, msgSN *uint32) {
	time.Sleep(2 * time.Second) // 等待2秒后发送注册信息

	reg := jtt809.VehicleRegistrationUpload{
		VehicleNo:         *vehicleNo,
		VehicleColor:      byte(*vehicleColor),
		PlatformID:        "Platform01",
		ProducerID:        "Manufacturer",
		TerminalModelType: "GPS-Model-X1",
		IMEI:              "860123456789012345", // 2019版本：30字节（实际内容可以较短，会补齐）
		TerminalID:        "TERM" + fmt.Sprintf("%d", *userID),
		TerminalSIM:       "13800138000",
	}

	pkg, err := jtt809.EncodePackage(jtt809.Package{
		Header: jtt809.Header{
			MsgSN:        *msgSN,
			BusinessType: jtt809.MsgIDDynamicInfo,
			Version:      jtt809.Version{Major: 1, Minor: 0, Patch: 0},
		},
		Body: reg,
	})
	if err != nil {
		log.Printf("Build vehicle registration failed: %v", err)
		return
	}
	*msgSN++

	log.Printf("[Main] Sending Vehicle Registration: Vehicle=%s, Color=%d", *vehicleNo, *vehicleColor)
	if _, err := conn.Write(pkg); err != nil {
		log.Printf("Send vehicle registration failed: %v", err)
	}
}

// sendLocationUpdates 定期发送GPS定位数据
func sendLocationUpdates(conn net.Conn, msgSN *uint32, interval time.Duration) {
	gps := NewGPSSimulator()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("[Main] Starting GPS location updates every %v", interval)

	for range ticker.C {
		position := gps.Next()

		upload := jtt809.VehicleLocationUpload{
			VehicleNo:    *vehicleNo,
			VehicleColor: byte(*vehicleColor),
			Position:     position,
		}

		pkg, err := jtt809.EncodePackage(jtt809.Package{
			Header: jtt809.Header{
				MsgSN:        *msgSN,
				BusinessType: jtt809.MsgIDDynamicInfo,
				Version:      jtt809.Version{Major: 1, Minor: 0, Patch: 0},
			},
			Body: upload,
		})
		if err != nil {
			log.Printf("Build location upload failed: %v", err)
			continue
		}
		*msgSN++

		log.Printf("[Main] Sending Location: Lon=%.6f, Lat=%.6f, Speed=%dkm/h, Direction=%d°",
			float64(position.Lon)/1000000.0, float64(position.Lat)/1000000.0, position.Speed, position.Direction)

		if _, err := conn.Write(pkg); err != nil {
			log.Printf("Send location update failed: %v", err)
			return
		}
	}
}
