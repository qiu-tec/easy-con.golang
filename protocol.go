/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2024/7/11 15:49
 */

package easyCon

import (
	"fmt"
	"sync/atomic"
	"time"
)

// EProtocol 协议
type EProtocol string

const (
	// EProtocolMQTT MQTT协议
	EProtocolMQTT EProtocol = "MQTT"

	EProtocolMQTTSync EProtocol = "MQTTSync"
	// EProtocolHTTP HTTP协议
	//EProtocolHTTP EProtocol = "HTTP"
)
const (
	NoticeTopic       string = "Notice"
	RetainNoticeTopic string = "RetainNotice"
	LogTopic          string = "Log"
)

//	type keyValuePair struct {
//		Key   string
//		Value any
//	}
type packBase struct {
	PType EPType
	Id    uint64
}

// PackReq 请求
type PackReq struct {
	packBase
	From    string
	ReqTime string
	To      string
	Route   string
	Content any
}

// PackResp 响应
type PackResp struct {
	PackReq
	RespTime string
	RespCode EResp
	Error    string
}
type PackLog struct {
	packBase
	From    string
	Level   ELogLevel
	LogTime string
	Error   string
	Content string
}
type PackNotice struct {
	packBase
	From    string
	Route   string
	Content any
}

// EPType 包类型枚举
type EPType string

const (
	// EPTypeReq 请求包
	EPTypeReq EPType = "REQ"
	// EPTypeResp 响应包
	EPTypeResp EPType = "RESP"
	// EPTypeNotice 通知包
	EPTypeNotice EPType = "NOTICE"
	// EPTypeLog 日志包
	EPTypeLog EPType = "LOG"
)

type EProxyMode string

const (
	// EProxyModeForward proxy Req from A -> B
	EProxyModeForward EProxyMode = "Forward"
	// EProxyModeReverse proxy Req from B -> A
	EProxyModeReverse EProxyMode = "Reverse"
	//// EProxyModeBoth proxy Req Both way
	//EProxyModeBoth EProxyMode = 2
)

// EStatus 访问器状态
type EStatus string

const (

	// EStatusConnecting 连接中
	EStatusConnecting EStatus = "Connecting"

	//EStatusLinked 已连接
	EStatusLinked EStatus = "Linked"

	// EStatusLinkLost 连接丢失
	EStatusLinkLost EStatus = "LinkLost"

	// EStatusFault 故障
	//EStatusFault EStatus = "Fault"

	// EStatusStopped 已停止
	EStatusStopped EStatus = "Stopped"
)

// EResp 请求响应
type EResp int

const (
	// ERespUnLinked 未连接
	ERespUnLinked EResp = 0
	// ERespSuccess 成功
	ERespSuccess EResp = 200
	// ERespBadReq 错误的请求
	ERespBadReq EResp = 400

	// ERespForbidden 权限不足
	//ERespForbidden EResp = 403

	// ERespRouteNotFind 路由未找到
	ERespRouteNotFind EResp = 404
	// ERespError 响应端故障
	ERespError EResp = 500
	// ERespTimeout 响应超时
	ERespTimeout EResp = 408
)

var (
	reqId          uint64
	logId          uint64
	noticeId       uint64
	retainNoticeId uint64
)

type ELogMode string

const (
	ELogModeNone    ELogMode = "NONE"
	ELogModeConsole ELogMode = "CONSOLE"
	// ELogModeFile    ELogMode = "FILE"

	ELogModeUpload ELogMode = "UPLOAD"
	ELogModeAll    ELogMode = "ALL"
)

type ELogLevel string

const (
	ELogLevelDebug   ELogLevel = "DEBUG"
	ELogLevelWarning ELogLevel = "WARNING"
	ELogLevelError   ELogLevel = "ERROR"
)

func getReqId() uint64 {
	return atomic.AddUint64(&reqId, 1)
}
func getLogId() uint64 {
	return atomic.AddUint64(&logId, 1)
}
func getNoticeId() uint64 {
	return atomic.AddUint64(&noticeId, 1)
}
func getRetainNoticeId() uint64 {
	return atomic.AddUint64(&retainNoticeId, 1)
}

// getRespCodeName 获取EResp枚举的名称
func getRespCodeName(code EResp) string {
	switch code {
	case ERespUnLinked:
		return "ERespUnLinked"
	case ERespSuccess:
		return "ERespSuccess"
	case ERespBadReq:
		return "ERespBadReq"
	case ERespRouteNotFind:
		return "ERespRouteNotFind"
	case ERespError:
		return "ERespError"
	case ERespTimeout:
		return "ERespTimeout"
	default:
		return fmt.Sprintf("Unknown(%d)", code)
	}
}

func newLogPack(module string, level ELogLevel, content string, err error) PackLog {
	eStr := ""
	if err != nil {
		eStr = err.Error()
	}
	return PackLog{
		packBase: packBase{
			PType: EPTypeLog,
			Id:    getLogId(),
		},
		LogTime: time.Now().Format("2006-01-02 15:04:05.000"),
		Level:   level,
		Content: content,
		Error:   eStr,
		From:    module,
	}
}

func getNowStr() string {
	return time.Now().Format("2006-01-02 15:04:05.000")
}

func newReqPack(from, to string, route string, content any) PackReq {
	return newReqPackInner(from, to, route, EPTypeReq, content)
}

func newReqPackInner(from, to string, route string, pType EPType, content any) PackReq {
	return PackReq{
		packBase: packBase{
			PType: pType,
			Id:    getReqId(),
		},
		From:    from,
		ReqTime: getNowStr(),
		To:      to,
		Route:   route,
		Content: content,
	}
}

func newRespPack(req PackReq, code EResp, content any) PackResp {
	pack := PackResp{
		PackReq:  req,
		RespTime: getNowStr(),
		RespCode: code,
	}
	pack.PType = EPTypeResp
	pack.Content = content
	if pack.RespCode != ERespSuccess && pack.Content != nil {
		if err, ok := content.(error); ok {
			pack.Error = err.Error()
		} else {
			pack.Error = fmt.Sprintf("%v", content)
		}
	}
	return pack
}

func newNoticePack(module, route string, content any) PackNotice {
	return PackNotice{
		packBase: packBase{
			PType: EPTypeNotice,
			Id:    getNoticeId(),
		},
		From:    module,
		Route:   route,
		Content: content,
	}
}
func newRetainNoticePack(module, route string, content any) PackNotice {
	return PackNotice{
		packBase: packBase{
			PType: EPTypeNotice,
			Id:    getRetainNoticeId(),
		},
		From:    module,
		Route:   route,
		Content: content,
	}
}
func buildReqTopic(module string) string {
	return "Request_" + module
}
func buildRespTopic(module string) string {
	return "Response_" + module
}
func buildGzipReqTopic(module string) string { return "RequestGzip_" + module } //ReqTopic
func buildGzipRespTopic(module string) string {
	return "ResponseGzip_" + module
}
