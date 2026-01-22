/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/12/25 14:12
 */

package easyCon

import (
	"fmt"
	"strings"
	"time"
)

type proxy struct {
	a IAdapter
	b IAdapter

	sa             CoreSetting
	sb             CoreSetting
	logForwardMode ELogForwardMode // 日志转发模式
	//mode              EProxyMode
	proxyModules      []string
	proxyNotice       bool
	proxyRetainNotice bool
	proxyLog          bool

	// A 端的请求回调，用于处理反向请求
	onReqRecA func(PackReq) (EResp, []byte)
}

func NewCgoMqttProxy(setting MqttProxySetting, onWrite func([]byte) error, aCallbacks AdapterCallBack) (IProxy, func([]byte)) {

	p := &proxy{
		//mode:              EProxyModeForward,
		proxyModules:      nil,
		proxyNotice:       true,
		proxyRetainNotice: true,
		proxyLog:          true,
		logForwardMode:    setting.LogForwardMode, // 默认为 NONE
		// 存储A端的OnReqRec回调，用于处理反向请求
		onReqRecA: aCallbacks.OnReqRec,
	}
	sa := CoreSetting{
		Module:            setting.Module,
		TimeOut:           setting.TimeOut,
		ReTry:             1, //setting.ReTry,
		LogMode:           ELogModeUpload,
		PreFix:            "",
		ChannelBufferSize: 100,
		ConnectRetryDelay: 0,
		IsWaitLink:        false,
		IsSync:            false,
	}
	a, f := NewCGoMonitorWithBroker(sa, AdapterCallBack{
		OnReqRec:          p.onReqA,
		OnNoticeRec:       p.onNoticeA,
		OnRetainNoticeRec: p.onRetainNoticeA,
		OnLogRec:          p.onLogA,
		OnRespRec:         nil,
	}, onWrite, onWrite) // Use onWrite for both external MQTT and local CgoBroker
	sb := NewDefaultMqttSetting(setting.Module, setting.Addr)
	sb.LogMode = ELogModeNone
	sb.PreFix = setting.PreFix
	sb.ReTry = 1 //setting.ReTry
	sb.TimeOut = setting.TimeOut
	sb.PWD = setting.PWD
	sb.UID = setting.UID

	b := NewMqttMonitor(sb, AdapterCallBack{
		OnReqRec:          p.onReqB,
		OnNoticeRec:       p.onNoticeB,
		OnRetainNoticeRec: p.onRetainNoticeB,
		OnLogRec:          p.onLogB,
		OnRespRec:         p.onRespB,
	})
	p.a = a
	p.b = b
	p.sa = sa
	p.sb = sb.CoreSetting
	return p, f
}

func (p *proxy) Stop() {
	p.a.Stop()
	p.b.Stop()
}

func (p *proxy) Reset() {
	p.a.Reset()
	p.b.Reset()
}

// onNoticeA 收到来自A的通知，无脑转发给B
func (p *proxy) onNoticeA(notice PackNotice) {
	if strings.HasPrefix(notice.From, p.sa.Module) { //来自自己
		return
	}
	// 修改From字段为 A/moduleA 格式
	notice.From = p.sb.Module + "/" + notice.From
	topic := BuildNoticeTopic(p.sb.PreFix, notice.Route)
	rawData, err := notice.Raw()
	if err != nil {
		p.b.Err("mqttProxy notice raw failed", err)
		return
	}

	err = p.b.PublishRaw(topic, false, rawData)
	if err != nil {
		p.b.Err(fmt.Sprintf("mqttProxy notice (%d) A->B failed", notice.Id), err)
	}
}

func (p *proxy) onRetainNoticeA(notice PackNotice) {
	if strings.HasPrefix(notice.From, p.sa.Module) { //来自自己
		return
	}
	// 修改From字段为 A/moduleA 格式
	notice.From = p.sb.Module + "/" + notice.From
	topic := BuildRetainNoticeTopic(p.sb.PreFix, notice.Route)

	rawData, err := notice.Raw()
	if err != nil {
		p.b.Err("mqttProxy retain notice raw failed", err)
		return
	}

	err = p.b.PublishRaw(topic, true, rawData)
	if err != nil {
		p.b.Err(fmt.Sprintf("mqttProxy retain notice (%d) A->B failed", notice.Id), err)
	}
}

func (p *proxy) onLogA(log PackLog) {
	// 根据LogForwardMode决定是否转发
	switch p.logForwardMode {
	case ELogForwardNone:
		return // 不转发日志
	case ELogForwardError:
		// 只转发ERROR级别的日志
		if log.Level != ELogLevelError {
			return
		}
	case ELogForwardAll:
		// 转发所有日志
	default:
		return
	}

	// 修改From字段为 A/moduleA 格式
	log.From = p.sb.Module + "/" + log.From

	topic := BuildLogTopic(p.sb.PreFix)

	rawData, err := log.Raw()
	if err != nil {
		p.b.Err("mqttProxy log raw failed", err)
		return
	}

	err = p.b.PublishRaw(topic, false, rawData)
	if err != nil {
		p.b.Err(fmt.Sprintf("mqttProxy log (%d) A->B failed", log.Id), err)
	}
}

func (p *proxy) onNoticeB(notice PackNotice) {
	if strings.HasPrefix(notice.From, p.sb.Module) { //来自自己
		return
	}
	// 注意：B->A的notice转发，From字段保持不变
	topic := BuildNoticeTopic(p.sa.PreFix, notice.Route)

	rawData, err := notice.Raw()
	if err != nil {
		p.b.Err("mqttProxy notice raw failed", err)
		return
	}

	err = p.a.PublishRaw(topic, false, rawData)
	if err != nil {
		p.b.Err(fmt.Sprintf("mqttProxy notice (%d) B->A failed", notice.Id), err)
	}
}

func (p *proxy) onRetainNoticeB(notice PackNotice) {
	if strings.HasPrefix(notice.From, p.sb.Module) { //来自自己
		return
	}
	// 注意：B->A的retain notice转发，From字段保持不变
	topic := BuildRetainNoticeTopic(p.sa.PreFix, notice.Route)

	rawData, err := notice.Raw()
	if err != nil {
		p.b.Err("mqttProxy retain notice raw failed", err)
		return
	}

	err = p.a.PublishRaw(topic, true, rawData)
	if err != nil {
		p.b.Err(fmt.Sprintf("mqttProxy retain notice (%d) B->A failed", notice.Id), err)
	}
}

func (p *proxy) onLogB(_ PackLog) {
	// 注意：Log从B到A不需要转发
	// 直接返回，不做任何处理
	return
}

// OnReqDetectedA 正向代理 让来自A的请求 转发给B
func (p *proxy) onReqA(pack PackReq) (EResp, []byte) {
	if pack.To == "Broker" {
		return ERespBypass, []byte{}
	}
	if strings.HasPrefix(pack.From, p.sa.Module) { //来自自己
		return ERespBypass, []byte{}
	}

	// 构建topic并检查是否为内部通信
	topic := BuildReqTopic(p.sb.PreFix, pack.To)

	if isInternalTopic(topic) { // 单层topic = 内部通信，不转发
		return ERespBypass, []byte{}
	}

	// 多层topic = 外部通信，需要转发并修改From
	// 修改From字段为 A/moduleA 格式
	modifiedPack := pack
	modifiedPack.From = p.sb.Module + "/" + pack.From
	modifiedData, err := modifiedPack.Raw()
	if err != nil {
		p.b.Err("mqttProxy req raw A->B failed", err)
		return ERespBypass, []byte{}
	}

	err = p.b.PublishRaw(topic, false, modifiedData)
	if err != nil {
		p.b.Err(fmt.Sprintf("mqttProxy req (%d) A->B failed", pack.Id), err)
		return ERespBypass, []byte{}
	}

	return ERespBypass, []byte{}
}

// onReqB 收到来自B的请求，转发给A
func (p *proxy) onReqB(pack PackReq) (EResp, []byte) {
	now := time.Now().Format("15:04:05.000")

	if strings.HasPrefix(pack.From, p.sb.Module) { //来自自己
		return ERespBypass, []byte{}
	}

	// 直接调用 A 端的回调来处理请求
	if p.onReqRecA == nil {
		fmt.Printf("[%s][Proxy-onReqB] ERROR: onReqRecA is nil, cannot process request\n", now)
		return ERespBypass, []byte{}
	}

	respCode, respContent := p.onReqRecA(pack)
	// 构建响应 pack
	respPack := PackResp{
		PackReq: PackReq{
			packBase: packBase{PType: EPTypeResp, Id: pack.Id},
			From:     pack.To,   // 响应的 From 是请求的目标
			To:       pack.From, // 响应的 To 是请求的发送者
			Route:    pack.Route,
			Content:  respContent,
		},
		RespCode: respCode,
	}

	// 发送响应到 B 端
	respTopic := BuildRespTopic(p.sb.PreFix, pack.From)
	rawData, err := respPack.Raw()
	if err != nil {
		p.b.Err("mqttProxy resp pack raw failed", err)
		return ERespBypass, []byte{}
	}

	err = p.b.PublishRaw(respTopic, false, rawData)
	if err != nil {
		p.b.Err(fmt.Sprintf("mqttProxy resp (%d) B->A failed", pack.Id), err)
		return ERespBypass, []byte{}
	}
	return ERespBypass, []byte{}
}

// isInternalTopic 判断topic是否为内部topic
// 单层topic (如 Request/ModuleA) = 内部，不需要转发
// 多层topic (如 Request/B/ModuleB) = 外部，需要转发
func isInternalTopic(topic string) bool {
	count := strings.Count(topic, "/")
	return count == 1 // 只有一个"/"表示单层topic，内部通信
}

// onRespA 收到来自A的响应，转发给B
func (p *proxy) onRespA(resp PackResp) {
	if strings.HasPrefix(resp.From, p.sa.Module) { //来自自己
		return
	}

	// 响应消息：resp.To 是目标（原始请求者），resp.From 是响应者
	// 使用 resp.To 作为响应目标
	topic := BuildRespTopic(p.sb.PreFix, resp.To)

	// 检查是否为内部通信
	if isInternalTopic(topic) {
		// 单层topic = 内部通信，不转发
		return
	}

	// 多层topic = 外部通信，需要转发
	// 注意：根据用户确认，Response的From不需要修改
	rawData, err := resp.Raw()
	if err != nil {
		p.b.Err("mqttProxy resp raw failed", err)
		return
	}

	err = p.b.PublishRaw(topic, false, rawData)
	if err != nil {
		p.b.Err(fmt.Sprintf("mqttProxy resp (%d) A->B failed", resp.Id), err)
	}
}

// onRespB 收到来自B的响应，转发给A
func (p *proxy) onRespB(resp PackResp) {
	if strings.HasPrefix(resp.From, p.sb.Module) { //来自自己
		return
	}

	// 检查响应是否发往A端（通过检查 resp.To 是否以 Proxy/ 开头）
	if !strings.HasPrefix(resp.To, p.sa.Module+"/") {
		return
	}

	// 响应消息：resp.To 是目标（原始请求者），resp.From 是响应者
	// 如果 resp.To 以 Proxy/ 开头，需要移除此前缀恢复原始目标
	targetTo := strings.TrimPrefix(resp.To, p.sa.Module+"/")

	// 修改响应的 To 字段，使其指向原始请求者
	modifiedResp := resp
	modifiedResp.To = targetTo

	// 使用修改后的响应构建 topic
	topic := BuildRespTopic(p.sa.PreFix, targetTo)

	// 序列化修改后的响应
	rawData, err := modifiedResp.Raw()
	if err != nil {
		p.b.Err("mqttProxy resp raw failed", err)
		return
	}

	err = p.a.PublishRaw(topic, false, rawData)
	if err != nil {
		p.b.Err(fmt.Sprintf("mqttProxy resp (%d) B->A failed", resp.Id), err)
	}
}
