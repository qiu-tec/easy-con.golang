/**
 * @Author: Joey
 * @Description: 协议类型定义
 * @Create Date: 2024/7/11 15:49
 */

package easyCon

// EProtocol 协议类型
type EProtocol string

// PackType 包类型常量
const (
	PackTypeReq    byte = 0x01
	PackTypeResp   byte = 0x02
	PackTypeNotice byte = 0x03
	PackTypeLog    byte = 0x04
)

// Topic 常量
const (
	NoticeTopic       string = "Notice"
	RetainNoticeTopic string = "RetainNotice"
	LogTopic          string = "Log"
	ReqTopic          string = "Request/"
	RespTopic         string = "Response/"
)

// PackBaseHeader 包基础头
type PackBaseHeader struct {
	PType EPType
	Id    uint64
}

// PackReqHeader 请求包头
type PackReqHeader struct {
	PackBaseHeader
	From    string
	ReqTime string
	To      string
	Route   string
}

// PackRespHeader 响应包头
type PackRespHeader struct {
	PackReqHeader
	RespTime string
	RespCode int
}

// PackNoticeHeader 通知包头
type PackNoticeHeader struct {
	PackBaseHeader
	From   string
	Route  string
	Retain bool
}

// PackLogHeader 日志包头
type PackLogHeader struct {
	PackBaseHeader
	From    string
	Level   string
	LogTime string
	Error   string
}

// EPType 包类型枚举
type EPType string

const (
	EPTypeReq    EPType = "REQ"
	EPTypeResp   EPType = "RESP"
	EPTypeNotice EPType = "NOTICE"
	EPTypeLog    EPType = "LOG"
)

// EProxyMode 代理模式
type EProxyMode string

// EStatus 状态枚举
type EStatus string

const (
	EStatusConnecting EStatus = "Connecting"
	EStatusLinked     EStatus = "Linked"
	EStatusLinkLost   EStatus = "LinkLost"
	EStatusStopped    EStatus = "Stopped"
)

// EResp 响应码枚举
type EResp int

const (
	ERespUnLinked     EResp = 0
	ERespSuccess      EResp = 200
	ERespBadReq       EResp = 400
	ERespRouteNotFind EResp = 404
	ERespError        EResp = 500
	ERespTimeout      EResp = 408
	ERespBypass       EResp = 100
)

// ELogMode 日志模式枚举
type ELogMode string

const (
	ELogModeNone    ELogMode = "NONE"
	ELogModeConsole ELogMode = "CONSOLE"
	ELogModeUpload  ELogMode = "UPLOAD"
	ELogModeAll     ELogMode = "ALL"
)

// ELogLevel 日志级别枚举
type ELogLevel string

const (
	ELogLevelDebug   ELogLevel = "DEBUG"
	ELogLevelWarning ELogLevel = "WARNING"
	ELogLevelError   ELogLevel = "ERROR"
)

// ELogForwardMode 日志转发模式枚举
type ELogForwardMode string

const (
	ELogForwardNone  ELogForwardMode = "NONE"  // 不转发日志
	ELogForwardError ELogForwardMode = "ERROR" // 只转发ERROR级别日志
	ELogForwardAll   ELogForwardMode = "ALL"   // 转发所有日志
)

// GetStatusName 获取状态名称
func GetStatusName(status EStatus) string {
	switch status {
	case EStatusStopped:
		return "Stopped"
	case EStatusLinkLost:
		return "LinkLost"
	case EStatusLinked:
		return "Linked"
	case EStatusConnecting:
		return "Connecting"
	}
	return "Unknown"
}
