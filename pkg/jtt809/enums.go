package jtt809

// PlateColor 表示车牌颜色类型（JT/T 809-2019）
type PlateColor byte

// 车牌颜色常量定义
const (
	PlateColorBlue          PlateColor = 0x01 // 蓝色
	PlateColorYellow        PlateColor = 0x02 // 黄色
	PlateColorBlack         PlateColor = 0x03 // 黑色
	PlateColorWhite         PlateColor = 0x04 // 白色
	PlateColorGreen         PlateColor = 0x05 // 绿色
	PlateColorOther         PlateColor = 0x09 // 其他
	PlateColorAgriYellow    PlateColor = 0x91 // 农黄色
	PlateColorYellowGreen   PlateColor = 0x93 // 黄绿色
	PlateColorAgriGreen     PlateColor = 0x92 // 农绿色
	PlateColorGradientGreen PlateColor = 0x94 // 渐变绿
)

// SubBusinessType 定义子业务数据类型，截取常用值以支持定位、查岗等业务。
const (
	// 上行子业务 (下级平台->上级平台)
	UP_EXG_MSG_REGISTER            uint16 = 0x1201 // 上传车辆注册信息
	UP_EXG_MSG_REAL_LOCATION       uint16 = 0x1202 // 实时上传车辆定位信息
	UP_EXG_MSG_HISTORY_LOCATION    uint16 = 0x1203 // 车辆定位信息自动补报
	UP_EXG_MSG_RETURN_STARTUP_ACK  uint16 = 0x1205 // 启动车辆定位信息交换应答
	UP_EXG_MSG_RETURN_END_ACK      uint16 = 0x1206 // 结束车辆定位信息交换应答
	UP_PLATFORM_MSG_POST_QUERY_ACK uint16 = 0x1301 // 平台查岗应答
	UP_WARN_MSG_ADPT_INFO          uint16 = 0x1402 // 上报报警信息
	UP_WARN_MSG_INFORM_TIPS        uint16 = 0x1403 // 上报报警预警消息

	// 下行子业务 (上级平台->下级平台)
	DOWN_EXG_MSG_RETURN_STARTUP uint16 = 0x9205 // 启动车辆定位信息交换请求
	DOWN_EXG_MSG_RETURN_END     uint16 = 0x9206 // 结束车辆定位信息交换请求
	DOWN_WARN_MSG_URGE_TODO_REQ uint16 = 0x9401 // 报警督办请求

	// JT/T 1078-2016 子业务
	UP_AUTHORIZE_MSG_STARTUP     uint16 = 0x1701 // 时效口令上报消息
	UP_REALVIDEO_MSG_STARTUP_ACK uint16 = 0x1801 // 实时音视频请求应答消息
	DOWN_REALVIDEO_MSG_STARTUP   uint16 = 0x9801 // 实时音视频请求消息
)

// MonitorReasonCode 启动/结束车辆定位信息交换请求原因
type MonitorReasonCode byte

const (
	MonitorReasonEnterArea MonitorReasonCode = 0x00 // 车辆进入指定区域
	MonitorReasonManual    MonitorReasonCode = 0x01 // 人工指定交换
	MonitorReasonEmergency MonitorReasonCode = 0x02 // 应急状态下车辆定位信息回传
	MonitorReasonOther     MonitorReasonCode = 0x03 // 其它原因
)

// WarnSrc 表示报警信息来源。
type WarnSrc byte

const (
	WarnSrcVehicle    WarnSrc = 0x01
	WarnSrcEnterprise WarnSrc = 0x02
	WarnSrcGovernment WarnSrc = 0x03
	WarnSrcOther      WarnSrc = 0x09
)

// WarnType 表示报警类型，结合表 3（位置相关）与表 4（非位置相关）常用编码。
type WarnType uint16

const (
	WarnTypeOverspeed              WarnType = 0x0001 // 超速报警
	WarnTypeFatigueDriving         WarnType = 0x0002 // 疲劳驾驶报警
	WarnTypeEmergency              WarnType = 0x0003 // 紧急报警
	WarnTypeEnterRegion            WarnType = 0x0004 // 进入指定区域报警
	WarnTypeLeaveRegion            WarnType = 0x0005 // 离开指定区域报警
	WarnTypeAcrossBoundary         WarnType = 0x0008 // 越界报警
	WarnTypeTheft                  WarnType = 0x0009 // 盗警
	WarnTypeHijack                 WarnType = 0x000A // 劫警
	WarnTypeRouteDeviation         WarnType = 0x000B // 偏离路线报警
	WarnTypeVehicleMove            WarnType = 0x000C // 车辆移动报警
	WarnTypeOvertimeDriving        WarnType = 0x000D // 超时驾驶报警
	WarnTypeViolationDriving       WarnType = 0x0010 // 违规行驶报警
	WarnTypeForwardCollision       WarnType = 0x0011 // 前撞报警
	WarnTypeLaneDeparture          WarnType = 0x0012 // 车道偏离报警
	WarnTypeTirePressureAbnormal   WarnType = 0x0013 // 胎压异常报警
	WarnTypeDynamicInfoAbnormal    WarnType = 0x0014 // 动态信息异常报警
	WarnTypeOther                  WarnType = 0x00FF // 其他报警
	WarnTypeTimeoutParking         WarnType = 0xA001 // 超时停车
	WarnTypeUploadIntervalAbnormal WarnType = 0xA002 // 车辆定位信息上报时间间隔异常
	WarnTypeUploadMileageAbnormal  WarnType = 0xA003 // 车辆定位信息上报里程间隔异常
	WarnTypeSubPlatformFreqAbn     WarnType = 0xA004 // 下级平台频率异常/断开
	WarnTypeSubPlatformTransmitAbn WarnType = 0xA005 // 下级平台数据传输异常
	WarnTypeRoadCongestion         WarnType = 0xA006 // 路段堵塞报警
	WarnTypeDangerousRoad          WarnType = 0xA007 // 危险路段报警
	WarnTypeBadWeather             WarnType = 0xA008 // 雨雪天气报警
	WarnTypeDriverIdAbnormal       WarnType = 0xA009 // 驾驶员身份识别异常
	WarnTypeTerminalAbnormal       WarnType = 0xA00A // 终端异常（含线路连接异常）
	WarnTypePlatformAccessAbn      WarnType = 0xA00B // 平台接入异常
	WarnTypeCoreDataAbnormal       WarnType = 0xA00C // 核心数据异常
	WarnTypeOtherNonLocation       WarnType = 0xA0FF // 其他报警
)

// SupervisionLevel 表示督办级别。
type SupervisionLevel byte

const (
	SupervisionLevelUrgent SupervisionLevel = 0x00
	SupervisionLevelNormal SupervisionLevel = 0x01
)

// DisconnectErrorCode 表示断开连接错误代码
type DisconnectErrorCode byte

const (
	// 0x1007 错误代码
	DisconnectMainLinkBroken DisconnectErrorCode = 0x00 // 主链路断开
	DisconnectOther          DisconnectErrorCode = 0x01 // 其他

	// 0x9007 错误代码
	DisconnectCannotConnectSub DisconnectErrorCode = 0x00 // 无法连接下级平台指定的服务 IP 与端口
	DisconnectSubLinkBroken    DisconnectErrorCode = 0x01 // 上级平台客户端与下级平台服务端断开
	DisconnectSubOther         DisconnectErrorCode = 0xFF // 其他
)
