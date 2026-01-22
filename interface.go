/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2024/7/10 16:17
 */

package easyCon

import (
	"time"
)

type ReqHandler func(pack PackReq) (EResp, []byte)
type OnReqHandler func(pack PackReq)
type RespHandler func(pack PackResp)
type NoticeHandler func(PackNotice)
type StatusChangedHandler func(status EStatus)
type LogHandler func(PackLog)
type PublishHandler func(topic string, isRetain bool, pack IPack) error
type PublishRawHandler func(topic string, isRetain bool, data []byte) error
type SubscribeHandler func(topic string, packType EPType, f func(IPack))

// EngineCallback 引擎回调
type EngineCallback struct {
	OnStop       func() (bool, error) //<-
	OnLink       func()
	OnSubscribe  SubscribeHandler
	OnPublish    PublishHandler
	OnPublishRaw PublishRawHandler
}
type AdapterCallBack struct {
	OnReqRec          ReqHandler
	OnRespRec         RespHandler
	OnNoticeRec       NoticeHandler
	OnRetainNoticeRec NoticeHandler
	OnLogRec          LogHandler
	OnExiting         func()
	OnGetVersion      func() []string
	OnLinked          func(adapter IAdapter)
	OnStatusChanged   StatusChangedHandler
}

// IAdapter 访问器接口
type IAdapter interface {
	Stop()

	Reset()

	Req(module, route string, params any) PackResp

	ReqWithTimeout(module, route string, params any, timeout int) PackResp

	SendNotice(route string, content any) error

	SubscribeNotice(route string, isRetain bool)

	SendRetainNotice(route string, content any) error

	CleanRetainNotice(route string) error

	Publish(topic string, isRetain bool, pack IPack) error

	PublishRaw(topic string, isRetain bool, data []byte) error

	GetEngineCallback() EngineCallback
	iLogger
}

type IProxy interface {
	// Stop 停止
	Stop()
	// Reset 复位
	Reset()
}
type iLogger interface {
	// Debug 发送调试信息
	Debug(content string)
	// Warn 发送警告
	Warn(content string)
	// Err 发送错误信息
	Err(content string, err error)
}

// MqttSetting 设置
type MqttSetting struct {
	CoreSetting
	// Addr 访问地址
	Addr string
	UID  string
	PWD  string
	// Mqtt特定配置
	MqttKeepAlive    time.Duration // MQTT keepalive间隔，默认30秒
	MqttPingTimeout  time.Duration // MQTT ping超时，默认10秒
	MqttWriteTimeout time.Duration // MQTT写入超时，默认无限制
}

// CoreSetting 设置
type CoreSetting struct {
	Module string
	// TimeOut 超时时间 毫秒
	TimeOut time.Duration
	// ReTry 请求重试次数
	ReTry int
	//SaveErrorLog bool
	LogMode ELogMode
	//PreFix 通用topic前缀 影响log notice
	PreFix string
	// ChannelBufferSize 各种消息通道的缓冲区大小
	ChannelBufferSize int
	// ConnectRetryDelay 连接重试之间的延迟
	ConnectRetryDelay time.Duration
	IsWaitLink        bool // IsWaitLink 等待连接
	// IsSync 是否同步
	IsSync bool
}

// MqttProxySetting 代理设置
type MqttProxySetting struct {
	Module string
	// Addr 访问地址
	Addr   string
	UID    string
	PWD    string
	PreFix string
	//ReTry   int
	TimeOut      time.Duration
	LogForwardMode ELogForwardMode // 日志转发模式
}

// NewDefaultMqttSetting 快速新建设置 默认3秒延迟 3次重试
func NewDefaultMqttSetting(module string, addr string) MqttSetting {
	return MqttSetting{
		CoreSetting: CoreSetting{
			Module: module,
			//EProtocol:         EProtocolMQTT,
			TimeOut: time.Second * 3,
			ReTry:   3,
			//SaveErrorLog:      false,
			LogMode:           ELogModeConsole,
			PreFix:            "",
			ChannelBufferSize: 100,         // 默认缓冲区大小
			ConnectRetryDelay: time.Second, // 默认重试延迟
			IsWaitLink:        true,
			//IsSync:            false,
		},
		Addr:             addr,
		MqttKeepAlive:    20 * time.Second, // WebSocket连接使用更短的keepalive
		MqttPingTimeout:  5 * time.Second,  // 默认10秒太长，缩短到5秒
		MqttWriteTimeout: 5 * time.Second,  // 设置5秒写入超时，避免无限阻塞
	}
}
