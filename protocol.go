/**
 * @Author: Joey
 * @Description: 协议序列化和工具函数
 * @Create Date: 2024/7/11 15:49
 */

package easyCon

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

var (
	reqId    uint64
	logId    uint64
	noticeId uint64
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

func getNowStr() string {
	return time.Now().Format("2006-01-02 15:04:05.000")
}

func newLogPack(module string, level ELogLevel, content string) PackLog {
	return PackLog{
		packBase: packBase{
			PType: EPTypeLog,
			Id:    getLogId(),
		},
		LogTime: time.Now().Format("2006-01-02 15:04:05.000"),
		Level:   level,
		Content: content,
		From:    module,
	}
}
func newReqPack(from, to string, route string, content []byte) PackReq {
	return newReqPackInner(from, to, route, EPTypeReq, content)
}

func newReqPackInner(from, to string, route string, pType EPType, content []byte) PackReq {
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

func newRespPack(req PackReq, code EResp, content []byte) PackResp {
	pack := PackResp{
		PackReq:  req,
		RespTime: getNowStr(),
		RespCode: code,
	}
	pack.PType = EPTypeResp

	// 对于响应，需要交换 From 和 To
	// req.From 是请求者，响应应该发送回请求者
	// req.To 是被请求的模块，应该是响应的发送者
	originalFrom := req.From
	originalTo := req.To
	pack.From = originalTo // 响应的发送者是被请求的模块
	pack.To = originalFrom // 响应的目标是原始请求者
	pack.Content = content
	return pack
}

func newNoticePack(module, route string, content []byte, isRetain bool) PackNotice {
	return PackNotice{
		packBase: packBase{
			PType: EPTypeNotice,
			Id:    getNoticeId(),
		},
		From:    module,
		Route:   route,
		Retain:  isRetain,
		Content: content,
	}
}

// ensureTrailingSlash 确保前缀以 / 结尾
func ensureTrailingSlash(prefix string) string {
	if prefix != "" && prefix[len(prefix)-1] != '/' {
		return prefix + "/"
	}
	return prefix
}

func BuildReqTopic(prefix, module string) string {
	return ensureTrailingSlash(prefix) + ReqTopic + module
}

func BuildRespTopic(prefix, module string) string {
	return ensureTrailingSlash(prefix) + RespTopic + module
}

func BuildNoticeTopic(prefix, sub string) string {
	return ensureTrailingSlash(prefix) + NoticeTopic + "/" + sub
}

func BuildRetainNoticeTopic(prefix, sub string) string {
	return ensureTrailingSlash(prefix) + RetainNoticeTopic + "/" + sub
}

func BuildLogTopic(prefix string) string {
	return ensureTrailingSlash(prefix) + LogTopic
}

func unmarshalPack(data []byte) (IPack, error) {
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
			Content:  string(contentBytes),
		}, nil

	default:
		return nil, fmt.Errorf("unknown pack type: 0x%02x", packType)
	}
}

// UnmarshalPack 反序列化数据包
func UnmarshalPack(data []byte) (IPack, error) {
	return unmarshalPack(data)
}
