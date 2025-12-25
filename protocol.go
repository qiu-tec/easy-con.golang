/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2024/7/11 15:49
 */

package easyCon

import (
	"encoding/json"
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

	//InternalNoticeTopic string = "InternalNotice"

	LogTopic  string = "Log"
	ReqTopic  string = "Request/"
	RespTopic string = "Response/"
)

type IPack interface {
	GetId() uint64
	GetType() EPType
	Target() string
	Raw() ([]byte, error)
	IsRetain() bool
}

// CommPack 通信专用包

type packBase struct {
	PType EPType
	Id    uint64
}

func (p *packBase) GetId() uint64 {
	return p.Id
}
func (p *packBase) GetType() EPType {
	return p.PType
}
func (p *packBase) Target() string {
	return ""
}
func (p *packBase) IsRetain() bool {
	return false
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

func (p *PackReq) Target() string {
	return p.To
}

func (p *PackResp) Target() string {
	return p.From
}
func serialize(p any) ([]byte, error) {
	d, err := json.Marshal(p)
	if err != nil {
		return nil, err
	} else {
		return d, err
	}
}
func deserialize(data []byte, p any) error {
	err := json.Unmarshal(data, p)
	return err
}
func (p *PackReq) Raw() ([]byte, error) {
	return serialize(p)
}

// PackResp 响应
type PackResp struct {
	PackReq
	RespTime string
	RespCode EResp
	Error    string
}

func (p *PackResp) Raw() ([]byte, error) {
	return serialize(p)
}

type PackLog struct {
	packBase
	From    string
	Level   ELogLevel
	LogTime string
	Error   string
	Content string
}

func (p *PackLog) Raw() ([]byte, error) {
	return serialize(p)
}

type PackNotice struct {
	packBase
	From    string
	Route   string
	Retain  bool
	Content any
}

func (p *PackNotice) Raw() ([]byte, error) {
	return serialize(p)
}
func (p *PackNotice) IsRetain() bool {
	return p.Retain
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
	// EProxyModeForward mqttProxy Req from A -> B
	EProxyModeForward EProxyMode = "Forward"
	// EProxyModeReverse mqttProxy Req from B -> A
	EProxyModeReverse EProxyMode = "Reverse"
	// EProxyModeBoth mqttProxy Req Both way
	EProxyModeBoth EProxyMode = "Both"
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

	ERespBypass EResp = 100
)

var (
	reqId    uint64
	logId    uint64
	noticeId uint64
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
func getReqId() uint64 {
	return atomic.AddUint64(&reqId, 1)
}
func getLogId() uint64 {
	return atomic.AddUint64(&logId, 1)
}
func getNoticeId() uint64 {
	return atomic.AddUint64(&noticeId, 1)
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
			pack.Content = nil
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

func BuildReqTopic(prefix, module string) string {
	if prefix != "" && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	return prefix + ReqTopic + module
}
func BuildRespTopic(prefix, module string) string {
	if prefix != "" && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	return prefix + RespTopic + module
}
func BuildNoticeTopic(prefix, sub string) string {
	if prefix != "" && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	return prefix + NoticeTopic + "/" + sub
}

func BuildRetainNoticeTopic(prefix, sub string) string {
	if prefix != "" && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	return prefix + RetainNoticeTopic + "/" + sub
}

func BuildLogTopic(prefix string) string {
	if prefix != "" && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	return prefix + LogTopic
}
func unmarshalPack(pType EPType, data []byte) (IPack, error) {
	switch pType {
	case EPTypeReq:
		var pack PackReq
		err := json.Unmarshal(data, &pack)
		if err != nil {
			return nil, err
		}
		return &pack, nil
	case EPTypeResp:
		var pack PackResp
		err := json.Unmarshal(data, &pack)
		if err != nil {
			return nil, err
		}
		return &pack, nil
	case EPTypeNotice:
		var pack PackNotice
		err := json.Unmarshal(data, &pack)
		if err != nil {
			return nil, err
		}
		return &pack, nil
	case EPTypeLog:
		var pack PackLog
		err := json.Unmarshal(data, &pack)
		if err != nil {
			return nil, err
		}
		return &pack, nil
	}
	return nil, fmt.Errorf("unknown pack type %s", pType)
}

//func BuildInternalNoticeTopic(prefix, route string) string {
//	if prefix != "" && prefix[len(prefix)-1] != '/' {
//		prefix += "/"
//	}
//	return prefix + NoticeTopic + "/" + route
//}
