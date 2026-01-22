/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/12/25 14:12
 */

package easyCon

import (
	"fmt"
	"strings"
	"sync"
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

	// 用于检测重复转发的 map
	forwardedRespIDs sync.Map  // map[uint64]bool

	// A 端的请求回调，用于处理反向请求
	onReqRecA func(PackReq) (EResp, []byte)
}

// NewCgoMqttProxyWithCallbacks 创建代理并指定A端的回调（支持反向请求）
func NewCgoMqttProxyWithCallbacks(setting MqttProxySetting, onWrite func([]byte) error, aCallbacks AdapterCallBack) (IProxy, func([]byte)) {
	return NewCgoMqttProxy(setting, onWrite, aCallbacks)
}

// NewCgoMqttProxy 创建代理（保持向后兼容）
// 注意：使用此函数创建的代理不支持反向请求
func NewCgoMqttProxyV1(setting MqttProxySetting, onWrite func([]byte) error) (IProxy, func([]byte)) {
	// 使用空的回调
	return NewCgoMqttProxy(setting, onWrite, AdapterCallBack{})
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
		onReqRecA:         aCallbacks.OnReqRec,
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
		// OnRespRec is intentionally nil to avoid loop:
		// The CGo side should only forward requests TO MQTT, not receive responses FROM CgoBroker
		// Responses come FROM MQTT broker (handled by MqttMonitor side)
		OnRespRec: nil,
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
	notice.From = p.sa.Module + "/" + notice.From
	topic := BuildNoticeTopic(p.sb.PreFix, notice.Route)

	rawData, err := notice.Raw()
	if err != nil {
		fmt.Printf("[%s]:mqttProxy notice raw failed: %s\r\n",
			time.Now().Format("15:04:05.000"), err.Error())
		return
	}

	err = p.b.PublishRaw(topic, false, rawData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy notice (%d) A->B failed: %s\r\n",
			time.Now().Format("15:04:05.000"), notice.Id, err.Error())
	}
}

func (p *proxy) onRetainNoticeA(notice PackNotice) {
	if strings.HasPrefix(notice.From, p.sa.Module) { //来自自己
		return
	}
	// 修改From字段为 A/moduleA 格式
	notice.From = p.sa.Module + "/" + notice.From
	topic := BuildRetainNoticeTopic(p.sb.PreFix, notice.Route)

	rawData, err := notice.Raw()
	if err != nil {
		fmt.Printf("[%s]:mqttProxy retain notice raw failed: %s\r\n",
			time.Now().Format("15:04:05.000"), err.Error())
		return
	}

	err = p.b.PublishRaw(topic, true, rawData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy retain notice (%d) A->B failed: %s\r\n",
			time.Now().Format("15:04:05.000"), notice.Id, err.Error())
	}
}

func (p *proxy) onLogA(log PackLog) {
	if strings.HasPrefix(log.From, p.sa.Module) { //来自自己
		return
	}

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
	log.From = p.sa.Module + "/" + log.From

	topic := BuildLogTopic(p.sb.PreFix)

	rawData, err := log.Raw()
	if err != nil {
		fmt.Printf("[%s]:mqttProxy log raw failed: %s\r\n",
			time.Now().Format("15:04:05.000"), err.Error())
		return
	}

	err = p.b.PublishRaw(topic, false, rawData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy log (%d) A->B failed: %s\r\n",
			time.Now().Format("15:04:05.000"), log.Id, err.Error())
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
		fmt.Printf("[%s]:mqttProxy notice raw failed: %s\r\n",
			time.Now().Format("15:04:05.000"), err.Error())
		return
	}

	err = p.a.PublishRaw(topic, false, rawData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy notice (%d) B->A failed: %s\r\n",
			time.Now().Format("15:04:05.000"), notice.Id, err.Error())
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
		fmt.Printf("[%s]:mqttProxy retain notice raw failed: %s\r\n",
			time.Now().Format("15:04:05.000"), err.Error())
		return
	}

	err = p.a.PublishRaw(topic, true, rawData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy retain notice (%d) B->A failed: %s\r\n",
			time.Now().Format("15:04:05.000"), notice.Id, err.Error())
	}
}

func (p *proxy) onLogB(log PackLog) {
	// 注意：Log从B到A不需要转发
	// 直接返回，不做任何处理
	return
}

// OnReqDetectedA 正向代理 让来自A的请求 转发给B
func (p *proxy) onReqA(pack PackReq) (EResp, []byte) {
	now := time.Now().Format("15:04:05.000")
	if pack.To == "Broker" {
		return ERespBypass, []byte{}
	}
	if strings.HasPrefix(pack.From, p.sa.Module) { //来自自己
		return ERespBypass, []byte{}
	}

	// 检查是否为 B->A 的请求（目标为 A 端模块）
	// B->A 的请求应该在 onReqB 中处理，不应该在 onReqA 中转发
	if strings.HasPrefix(pack.To, "A/") {
		fmt.Printf("[%s][Proxy-onReqA] SKIP: B->A request (To=%s), handled by onReqB\n", now, pack.To)
		return ERespBypass, []byte{}
	}

	// 构建topic并检查是否为内部通信
	topic := BuildReqTopic(p.sb.PreFix, pack.To)
	fmt.Printf("[%s][Proxy-onReqA] Received: ID=%d From=%s To=%s topic=%s\n", now, pack.Id, pack.From, pack.To, topic)

	if isInternalTopic(topic) {
		// 单层topic = 内部通信，不转发
		fmt.Printf("[%s][Proxy-onReqA] SKIP: Internal topic\n", now)
		return ERespBypass, []byte{}
	}

	// 多层topic = 外部通信，需要转发并修改From
	// 修改From字段为 A/moduleA 格式
	modifiedPack := pack
	modifiedPack.From = p.sa.Module + "/" + pack.From
	modifiedData, err := modifiedPack.Raw()
	if err != nil {
		fmt.Printf("[%s]:mqttProxy modified pack raw failed: %s\r\n", now, err.Error())
		return ERespError, []byte{}
	}

	// 直接发布修改后的字节
	fmt.Printf("[%s][Proxy-onReqA] FORWARD: ID=%d From=%s -> topic=%s\n", now, pack.Id, modifiedPack.From, topic)
	err = p.b.PublishRaw(topic, false, modifiedData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy req (%d) A->B failed: %s\r\n", now, pack.Id, err.Error())
		return ERespError, []byte{}
	}

	return ERespBypass, []byte{}
}

// onReqB 收到来自B的请求，转发给A
func (p *proxy) onReqB(pack PackReq) (EResp, []byte) {
	now := time.Now().Format("15:04:05.000")

	if strings.HasPrefix(pack.From, p.sb.Module) { //来自自己
		return ERespBypass, []byte{}
	}

	// 检查 pack.To 是否为 A 端模块（以 A/ 开头的多层模块名）
	// B 端的模块名是 B/ModuleC，A 端的模块名是 A/ModuleA
	// 如果 pack.To 以 A/ 开头，说明是发往 A 端的请求
	if !strings.HasPrefix(pack.To, "A/") {
		// 不是发往A端的请求，不转发
		fmt.Printf("[%s][Proxy-onReqB] SKIP: Request not for A side (To=%s)\n", now, pack.To)
		return ERespBypass, []byte{}
	}

	// 这是发给 A 端的反向请求
	fmt.Printf("[%s][Proxy-onReqB] Received reverse request: ID=%d From=%s To=%s\n", now, pack.Id, pack.From, pack.To)

	// 直接调用 A 端的回调来处理请求
	if p.onReqRecA == nil {
		fmt.Printf("[%s][Proxy-onReqB] ERROR: onReqRecA is nil, cannot process request\n", now)
		return ERespError, []byte{}
	}

	respCode, respContent := p.onReqRecA(pack)
	fmt.Printf("[%s][Proxy-onReqB] Called onReqRecA: ID=%d RespCode=%d\n", now, pack.Id, respCode)

	// 构建响应 pack
	respPack := PackResp{
		PackReq: PackReq{
			packBase: packBase{PType: EPTypeResp, Id: pack.Id},
			From:     pack.To,    // 响应的 From 是请求的目标
			To:       pack.From,  // 响应的 To 是请求的发送者
			Route:    pack.Route,
			Content:  respContent,
		},
		RespCode: respCode,
	}

	// 发送响应到 B 端
	respTopic := BuildRespTopic(p.sb.PreFix, pack.From)
	rawData, err := respPack.Raw()
	if err != nil {
		fmt.Printf("[%s]:mqttProxy resp pack raw failed: %s\r\n", now, err.Error())
		return ERespError, []byte{}
	}

	err = p.b.PublishRaw(respTopic, false, rawData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy resp (%d) B->A failed: %s\r\n", now, pack.Id, err.Error())
		return ERespError, []byte{}
	}

	fmt.Printf("[%s][Proxy-onReqB] Sent response: ID=%d topic=%s\n", now, pack.Id, respTopic)
	return ERespBypass, []byte{}
}

func getMonitorTopic(topic string) string {
	sections := strings.Split(topic, "/")
	sections = sections[:len(sections)-1]
	return strings.Join(sections, "/") + "/#"
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
		fmt.Printf("[%s]:mqttProxy resp raw failed: %s\r\n",
			time.Now().Format("15:04:05.000"), err.Error())
		return
	}

	err = p.b.PublishRaw(topic, false, rawData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy resp (%d) A->B failed: %s\r\n",
			time.Now().Format("15:04:05.000"), resp.Id, err.Error())
	}
}

// onRespB 收到来自B的响应，转发给A
func (p *proxy) onRespB(resp PackResp) {
	now := time.Now().Format("15:04:05.000")

	// 检查是否已经转发过此响应 ID
	if _, alreadyForwarded := p.forwardedRespIDs.Load(resp.Id); alreadyForwarded {
		fmt.Printf("[%s][Proxy-onRespB] SKIP: ID=%d already forwarded\n", now, resp.Id)
		return
	}

	if strings.HasPrefix(resp.From, p.sb.Module) { //来自自己
		fmt.Printf("[%s][Proxy-onRespB] SKIP: Response from self (From=%s)\n", now, resp.From)
		return
	}

	// 检查响应是否发往A端（通过检查 resp.To 是否以 Proxy/ 开头）
	// 因为我们只在这个方向修改了 From，所以如果 To 以 Proxy/ 开头，就说明是A端的请求
	if !strings.HasPrefix(resp.To, p.sa.Module+"/") {
		fmt.Printf("[%s][Proxy-onRespB] SKIP: Response not for A side (To=%s)\n", now, resp.To)
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

	// 标记此 ID 已转发
	p.forwardedRespIDs.Store(resp.Id, true)

	fmt.Printf("[%s][Proxy-onRespB] FORWARD: ID=%d From=%s To=%s -> topic=%s (original To=%s)\n", now, resp.Id, resp.From, targetTo, topic, resp.To)

	// 序列化修改后的响应
	rawData, err := modifiedResp.Raw()
	if err != nil {
		fmt.Printf("[%s]:mqttProxy resp raw failed: %s\r\n", now, err.Error())
		return
	}

	err = p.a.PublishRaw(topic, false, rawData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy resp (%d) B->A failed: %s\r\n", now, resp.Id, err.Error())
	} else {
		fmt.Printf("[%s][Proxy-onRespB] SUCCESS: ID=%d forwarded\n", now, resp.Id)
	}
}
