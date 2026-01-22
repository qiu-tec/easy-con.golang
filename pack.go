/**
 * @Author: Joey
 * @Description: 数据包结构定义
 * @Create Date: 2024/7/11 15:49
 */

package easyCon

import (
	"encoding/json"
)

// IPack 数据包接口
type IPack interface {
	GetId() uint64
	GetType() EPType
	Target() string
	Raw() ([]byte, error)
	IsRetain() bool
}

// packBase 数据包基础结构
type packBase struct {
	PType EPType
	Id    uint64
}

func (p *packBase) GetId() uint64   { return p.Id }
func (p *packBase) GetType() EPType { return p.PType }
func (p *packBase) Target() string  { return "" }
func (p *packBase) IsRetain() bool  { return false }

// PackReq 请求数据包
type PackReq struct {
	packBase
	From    string
	ReqTime string
	To      string
	Route   string
	Content []byte
}

func (p *PackReq) Target() string { return p.To }

func (p *PackReq) Raw() ([]byte, error) {
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

	contentBytes := p.Content
	if contentBytes == nil {
		contentBytes = []byte{}
	}

	headLen := len(headerJson)
	totalLen := 3 + headLen + len(contentBytes)

	buf := make([]byte, totalLen)
	buf[0] = PackTypeReq
	buf[1] = byte(headLen >> 8)
	buf[2] = byte(headLen)
	copy(buf[3:], headerJson)
	copy(buf[3+headLen:], contentBytes)

	return buf, nil
}

// PackResp 响应数据包
type PackResp struct {
	PackReq
	RespTime string
	RespCode EResp
}

func (p *PackResp) Target() string { return p.From }

func (p *PackResp) Raw() ([]byte, error) {
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
	}
	headerJson, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	contentBytes := p.Content
	if contentBytes == nil {
		contentBytes = []byte{}
	}

	headLen := len(headerJson)
	totalLen := 3 + headLen + len(contentBytes)

	buf := make([]byte, totalLen)
	buf[0] = PackTypeResp
	buf[1] = byte(headLen >> 8)
	buf[2] = byte(headLen)
	copy(buf[3:], headerJson)
	copy(buf[3+headLen:], contentBytes)

	return buf, nil
}

// PackLog 日志数据包
type PackLog struct {
	packBase
	From    string
	Level   ELogLevel
	LogTime string
	Content string
}

func (p *PackLog) Raw() ([]byte, error) {
	header := PackLogHeader{
		PackBaseHeader: PackBaseHeader{
			PType: p.PType,
			Id:    p.Id,
		},
		From:    p.From,
		Level:   string(p.Level),
		LogTime: p.LogTime,
	}
	headerJson, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	contentBytes := ([]byte)(p.Content)
	headLen := len(headerJson)
	totalLen := 3 + headLen + len(contentBytes)

	buf := make([]byte, totalLen)
	buf[0] = PackTypeLog
	buf[1] = byte(headLen >> 8)
	buf[2] = byte(headLen)
	copy(buf[3:], headerJson)
	copy(buf[3+headLen:], contentBytes)
	return buf, nil
}

// PackNotice 通知数据包
type PackNotice struct {
	packBase
	From    string
	Route   string
	Retain  bool
	Content []byte
}

func (p *PackNotice) IsRetain() bool { return p.Retain }

func (p *PackNotice) Raw() ([]byte, error) {
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

	contentBytes := p.Content
	if contentBytes == nil {
		contentBytes = []byte{}
	}

	headLen := len(headerJson)
	totalLen := 3 + headLen + len(contentBytes)

	buf := make([]byte, totalLen)
	buf[0] = PackTypeNotice
	buf[1] = byte(headLen >> 8)
	buf[2] = byte(headLen)
	copy(buf[3:], headerJson)
	copy(buf[3+headLen:], contentBytes)

	return buf, nil
}
