/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2024/7/10 16:17
 */

package easyCon

import (
	"time"
)

type ReqHandler func(pack PackReq) (EResp, any)
type EventHandler func(EResp, any)
type StatusChangedHandler func(adapter IAdapter, status EStatus)
type EventBoundHandler func(string)

// IAdapter 访问器接口
type IAdapter interface {
	//// Init 初始化
	//Init(setting Setting)

	// Stop 停止
	Stop()
	// Reset 复位
	Reset()
	// Req 请求
	Req(module, route string, params any) PackResp

	SendNotice(any) error
	iLogger
}
type iLogger interface {
	// Debug 发送调试信息
	Debug(content string)
	// Warn 发送警告
	Warn(content string)
	// Err 发送错误信息
	Err(content string, err error)
}

// Setting 设置
type Setting struct {
	Module string
	// EProtocol 协议
	EProtocol EProtocol
	// Addr 访问地址
	Addr string
	// TimeOut 超时时间 毫秒
	TimeOut time.Duration
	// ReTry 请求重试次数
	ReTry         int
	OnReq         ReqHandler
	StatusChanged StatusChangedHandler
	UID           string
	PWD           string
	SaveErrorLog  bool
	LogMode       ELogMode
}

// NewSetting 快速新建设置 默认3秒延迟 3次重试
func NewSetting(module string, addr string, onReq ReqHandler, onStatusChanged StatusChangedHandler) Setting {
	return Setting{
		Module:        module,
		EProtocol:     EProtocolMQTT,
		Addr:          addr,
		TimeOut:       time.Second * 3,
		ReTry:         3,
		OnReq:         onReq,
		StatusChanged: onStatusChanged,
		LogMode:       ELogModeConsole,
	}
}
