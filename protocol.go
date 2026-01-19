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

// PackType 常量定义
const (
	PackTypeReq    byte = 0x01
	PackTypeResp   byte = 0x02
	PackTypeNotice byte = 0x03
	PackTypeLog    byte = 0x04
	PackTypeOld    byte = 0x7B // '{' 用于识别旧协议
)

// 新协议的 Header 结构体
type PackBaseHeader struct {
	PType EPType
	Id    uint64
}

type PackReqHeader struct {
	PackBaseHeader
	From    string
	ReqTime string
	To      string
	Route   string
}

type PackRespHeader struct {
	PackReqHeader
	RespTime string
	RespCode int
	Error    string
}

type PackNoticeHeader struct {
	PackBaseHeader
	From  string
	Route string
	Retain bool
}

type PackLogHeader struct {
	PackBaseHeader
	From    string
	Level   string
	LogTime string
	Error   string
	Content string // 日志的 content 在 header 里
}

// const (
//
//	// EProtocolMQTT MQTT协议
//	EProtocolMQTT EProtocol = "MQTT"
//
//	EProtocolMQTTSync EProtocol = "MQTTSync"
//	// EProtocolHTTP HTTP协议
//	//EProtocolHTTP EProtocol = "HTTP"
//
// )
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
	Content []byte
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
	// 1. 构建 Header JSON
	header := PackReqHeader{
		PackBaseHeader: PackBaseHeader{
			PType: p.PType,
			Id:    p.Id,
		},
		From:    p.From,
		ReqTime: p.ReqTime,
		To:      p.To,
		Route:   p.Route,
	}
	headerJson, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	// 2. Content 已经是 []byte
	contentBytes := p.Content
	if contentBytes == nil {
		contentBytes = []byte{}
	}

	// 3. 打包: [PackType][HeadLen(2)][Header JSON][Content]
	headLen := len(headerJson)
	totalLen := 3 + headLen + len(contentBytes)

	buf := make([]byte, totalLen)
	buf[0] = PackTypeReq
	buf[1] = byte(headLen >> 8)   // 大端序
	buf[2] = byte(headLen)
	copy(buf[3:], headerJson)
	copy(buf[3+headLen:], contentBytes)

	return buf, nil
}

// PackResp 响应
type PackResp struct {
	PackReq
	RespTime string
	RespCode EResp
	Error    string
}

func (p *PackResp) Raw() ([]byte, error) {
	// 1. 构建 Header JSON
	header := PackRespHeader{
		PackReqHeader: PackReqHeader{
			PackBaseHeader: PackBaseHeader{
				PType: p.PType,
				Id:    p.Id,
			},
			From:    p.From,
			ReqTime: p.ReqTime,
			To:      p.To,
			Route:   p.Route,
		},
		RespTime: p.RespTime,
		RespCode: int(p.RespCode),
		Error:    p.Error,
	}
	headerJson, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	// 2. Content 已经是 []byte
	contentBytes := p.Content
	if contentBytes == nil {
		contentBytes = []byte{}
	}

	// 3. 打包: [PackType][HeadLen(2)][Header JSON][Content]
	headLen := len(headerJson)
	totalLen := 3 + headLen + len(contentBytes)

	buf := make([]byte, totalLen)
	buf[0] = PackTypeResp
	buf[1] = byte(headLen >> 8)   // 大端序
	buf[2] = byte(headLen)
	copy(buf[3:], headerJson)
	copy(buf[3+headLen:], contentBytes)

	return buf, nil
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
	// 1. 构建 Header JSON (Content 在 header 里)
	header := PackLogHeader{
		PackBaseHeader: PackBaseHeader{
			PType: p.PType,
			Id:    p.Id,
		},
		From:    p.From,
		Level:   string(p.Level),
		LogTime: p.LogTime,
		Error:   p.Error,
		Content: p.Content, // 日志的 content 在 header 里
	}
	headerJson, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	// 2. PackLog 没有 Content，使用空 []byte
	contentBytes := []byte{}

	// 3. 打包: [PackType][HeadLen(2)][Header JSON][Content]
	headLen := len(headerJson)
	totalLen := 3 + headLen + len(contentBytes)

	buf := make([]byte, totalLen)
	buf[0] = PackTypeLog
	buf[1] = byte(headLen >> 8)   // 大端序
	buf[2] = byte(headLen)
	copy(buf[3:], headerJson)
	copy(buf[3+headLen:], contentBytes)

	return buf, nil
}

type PackNotice struct {
	packBase
	From    string
	Route   string
	Retain  bool
	Content []byte
}

func (p *PackNotice) Raw() ([]byte, error) {
	// 1. 构建 Header JSON
	header := PackNoticeHeader{
		PackBaseHeader: PackBaseHeader{
			PType: p.PType,
			Id:    p.Id,
		},
		From:   p.From,
		Route:  p.Route,
		Retain: p.Retain,
	}
	headerJson, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	// 2. Content 已经是 []byte
	contentBytes := p.Content
	if contentBytes == nil {
		contentBytes = []byte{}
	}

	// 3. 打包: [PackType][HeadLen(2)][Header JSON][Content]
	headLen := len(headerJson)
	totalLen := 3 + headLen + len(contentBytes)

	buf := make([]byte, totalLen)
	buf[0] = PackTypeNotice
	buf[1] = byte(headLen >> 8)   // 大端序
	buf[2] = byte(headLen)
	copy(buf[3:], headerJson)
	copy(buf[3+headLen:], contentBytes)

	return buf, nil
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

//const (
//	// EProxyModeForward mqttProxy Req from A -> B
//	EProxyModeForward EProxyMode = "Forward"
//	// EProxyModeReverse mqttProxy Req from B -> A
//	EProxyModeReverse EProxyMode = "Reverse"
//	// EProxyModeBoth mqttProxy Req Both way
//	EProxyModeBoth EProxyMode = "Both"
//)

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

//// getRespCodeName 获取EResp枚举的名称
//func getRespCodeName(code EResp) string {
//	switch code {
//	case ERespUnLinked:
//		return "ERespUnLinked"
//	case ERespSuccess:
//		return "ERespSuccess"
//	case ERespBadReq:
//		return "ERespBadReq"
//	case ERespRouteNotFind:
//		return "ERespRouteNotFind"
//	case ERespError:
//		return "ERespError"
//	case ERespTimeout:
//		return "ERespTimeout"
//	default:
//		return fmt.Sprintf("Unknown(%d)", code)
//	}
//}

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
	var contentBytes []byte
	if content != nil {
		// 如果已经是 []byte，直接使用
		if bytes, ok := content.([]byte); ok {
			contentBytes = bytes
		} else {
			// 否则序列化为 JSON
			var err error
			contentBytes, err = json.Marshal(content)
			if err != nil {
				contentBytes = []byte{}
			}
		}
	} else {
		contentBytes = []byte{}
	}
	return PackReq{
		packBase: packBase{
			PType: pType,
			Id:    getReqId(),
		},
		From:    from,
		ReqTime: getNowStr(),
		To:      to,
		Route:   route,
		Content: contentBytes,
	}
}

func newRespPack(req PackReq, code EResp, content any) PackResp {
	pack := PackResp{
		PackReq:  req,
		RespTime: getNowStr(),
		RespCode: code,
	}
	pack.PType = EPTypeResp

	// 在序列化之前处理 error 类型
	if code != ERespSuccess && content != nil {
		if err, ok := content.(error); ok {
			pack.Error = err.Error()
			content = nil // 设置为 nil，序列化为空
		} else if str, ok := content.(string); ok {
			pack.Error = str
			content = nil
		}
	}

	// 处理 content
	if content != nil {
		// 如果已经是 []byte，直接使用
		if bytes, ok := content.([]byte); ok {
			pack.Content = bytes
		} else {
			// 否则序列化为 JSON
			var err error
			pack.Content, err = json.Marshal(content)
			if err != nil {
				pack.Content = []byte{}
			}
		}
	} else {
		pack.Content = []byte{}
	}

	return pack
}

func newNoticePack(module, route string, content any) PackNotice {
	var contentBytes []byte
	if content != nil {
		// 如果已经是 []byte，直接使用
		if bytes, ok := content.([]byte); ok {
			contentBytes = bytes
		} else {
			// 否则序列化为 JSON
			var err error
			contentBytes, err = json.Marshal(content)
			if err != nil {
				contentBytes = []byte{}
			}
		}
	} else {
		contentBytes = []byte{}
	}
	return PackNotice{
		packBase: packBase{
			PType: EPTypeNotice,
			Id:    getNoticeId(),
		},
		From:    module,
		Route:   route,
		Content: contentBytes,
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
func unmarshalPackOld(pType EPType, data []byte) (IPack, error) {
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

func unmarshalPackNew(data []byte) (IPack, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("invalid pack length: %d", len(data))
	}

	packType := data[0]
	headLen := int(data[1])<<8 | int(data[2])

	if len(data) < 3+headLen {
		return nil, fmt.Errorf("invalid header length: dataLen=%d, headLen=%d", len(data), headLen)
	}

	headerBytes := data[3 : 3+headLen]
	contentBytes := data[3+headLen:]

	switch packType {
	case PackTypeReq:
		var header PackReqHeader
		if err := json.Unmarshal(headerBytes, &header); err != nil {
			return nil, fmt.Errorf("failed to unmarshal REQ header: %w", err)
		}
		return &PackReq{
			packBase: packBase{PType: header.PType, Id: header.Id},
			From:     header.From,
			To:       header.To,
			Route:    header.Route,
			ReqTime:  header.ReqTime,
			Content:  contentBytes,
		}, nil

	case PackTypeResp:
		var header PackRespHeader
		if err := json.Unmarshal(headerBytes, &header); err != nil {
			return nil, fmt.Errorf("failed to unmarshal RESP header: %w", err)
		}
		return &PackResp{
			PackReq: PackReq{
				packBase: packBase{PType: header.PType, Id: header.Id},
				From:     header.From,
				To:       header.To,
				Route:    header.Route,
				ReqTime:  header.ReqTime,
				Content:  contentBytes,
			},
			RespTime: header.RespTime,
			RespCode: EResp(header.RespCode),
			Error:    header.Error,
		}, nil

	case PackTypeNotice:
		var header PackNoticeHeader
		if err := json.Unmarshal(headerBytes, &header); err != nil {
			return nil, fmt.Errorf("failed to unmarshal NOTICE header: %w", err)
		}
		return &PackNotice{
			packBase: packBase{PType: header.PType, Id: header.Id},
			From:     header.From,
			Route:    header.Route,
			Retain:   header.Retain,
			Content:  contentBytes,
		}, nil

	case PackTypeLog:
		var header PackLogHeader
		if err := json.Unmarshal(headerBytes, &header); err != nil {
			return nil, fmt.Errorf("failed to unmarshal LOG header: %w", err)
		}
		return &PackLog{
			packBase: packBase{PType: header.PType, Id: header.Id},
			From:     header.From,
			Level:    ELogLevel(header.Level),
			LogTime:  header.LogTime,
			Error:    header.Error,
			Content:  header.Content,
		}, nil

	default:
		return nil, fmt.Errorf("unknown pack type: 0x%02x", packType)
	}
}

// UnmarshalPack 统一的反序列化入口，自动识别新旧协议
func UnmarshalPack(data []byte) (IPack, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	// 根据第一个字节判断协议类型
	if data[0] == PackTypeOld { // '{'
		// 旧协议：需要先读取 PType 字段来确定类型
		var typeOnly struct {
			PType EPType `json:"PType"`
		}
		if err := json.Unmarshal(data, &typeOnly); err != nil {
			return nil, fmt.Errorf("failed to detect old protocol type: %w", err)
		}
		return unmarshalPackOld(typeOnly.PType, data)
	}

	// 新协议
	return unmarshalPackNew(data)
}

//func BuildInternalNoticeTopic(prefix, route string) string {
//	if prefix != "" && prefix[len(prefix)-1] != '/' {
//		prefix += "/"
//	}
//	return prefix + NoticeTopic + "/" + route
//}
