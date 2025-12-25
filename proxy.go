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

	sa                CoreSetting
	sb                CoreSetting
	mode              EProxyMode
	proxyModules      []string
	proxyNotice       bool
	proxyRetainNotice bool
	proxyLog          bool
}

func NewCgoMqttProxy(setting MqttProxySetting, onWrite func([]byte) error) (IProxy, func([]byte)) {

	p := &proxy{
		mode:              EProxyModeForward,
		proxyModules:      nil,
		proxyNotice:       true,
		proxyRetainNotice: true,
		proxyLog:          true,
	}
	sa := CoreSetting{
		Module:            "#",
		TimeOut:           0,
		ReTry:             0,
		LogMode:           ELogModeUpload,
		PreFix:            "",
		ChannelBufferSize: 100,
		ConnectRetryDelay: 0,
		IsWaitLink:        false,
		IsSync:            false,
	}
	a, f := NewCGoMonitor(sa, AdapterCallBack{
		OnReqRec:          p.onReqA,
		OnNoticeRec:       p.onNoticeA,
		OnRetainNoticeRec: p.onRetainNoticeA,
		OnLogRec:          p.onLogA,
	}, onWrite)
	sb := NewDefaultMqttSetting("#", setting.Addr)
	sb.LogMode = ELogModeNone
	sb.PreFix = setting.PreFix
	sb.ReTry = setting.ReTry
	sb.TimeOut = setting.TimeOut
	sb.PWD = setting.PWD
	sb.UID = setting.UID

	b := NewMqttAdapter(sb, AdapterCallBack{
		OnReqRec:          p.onReqB,
		OnNoticeRec:       p.onNoticeB,
		OnRetainNoticeRec: p.onRetainNoticeB,
		OnLogRec:          p.onLogB,
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

// onNoticeA 收到来自A的通知
func (m *proxy) onNoticeA(notice PackNotice) {
	if m.mode == EProxyModeForward || !m.proxyNotice || m.b == nil {
		return
	}

	topic := BuildNoticeTopic(m.sb.PreFix, notice.Route)
	err := m.b.Publish(topic, false, &notice)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy notice  (%d) route %s  A-> B Failed because %s\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Route, err.Error())
		return
	}

}

func (m *proxy) onRetainNoticeA(notice PackNotice) {
	if m.mode == EProxyModeForward || !m.proxyRetainNotice || m.b == nil {
		return
	}
	topic := BuildRetainNoticeTopic(m.sb.PreFix, notice.Route)
	err := m.b.Publish(topic, true, &notice)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy retain notice  (%d) route %s  A-> B Failed because %s\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Route, err.Error())
		return
	}
}

func (m *proxy) onLogA(log PackLog) {
	if m.mode == EProxyModeForward || !m.proxyLog || m.b == nil {
		return
	}
	topic := BuildLogTopic(m.sb.PreFix)
	err := m.b.Publish(topic, false, &log)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy log  (%d) A-> B Failed because %s\r\n", time.Now().Format("15:04:05.000"), log.Id, err.Error())
		return
	}
}

func (m *proxy) onNoticeB(notice PackNotice) {
	if m.mode == EProxyModeReverse || !m.proxyNotice || m.a == nil {
		return
	}
	topic := BuildNoticeTopic(m.sa.PreFix, notice.Route)
	err := m.a.Publish(topic, false, &notice)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy notice  (%d) route %s  B-> A Failed because %s\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Route, err.Error())
		return
	}
}

func (m *proxy) onRetainNoticeB(notice PackNotice) {
	if m.mode == EProxyModeReverse || !m.proxyRetainNotice || m.a == nil {
		return
	}
	topic := BuildRetainNoticeTopic(m.sa.PreFix, notice.Route)
	err := m.a.Publish(topic, true, &notice)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy retain notice  (%d) route %s  B-> A Failed because %s\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Route, err.Error())
		return
	}
}

func (m *proxy) onLogB(log PackLog) {
	if m.mode == EProxyModeReverse || !m.proxyLog || m.a == nil {
		return
	}
	topic := BuildLogTopic(m.sa.PreFix)
	err := m.a.Publish(topic, false, &log)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy log  (%d)  B-> A Failed because %s\r\n", time.Now().Format("15:04:05.000"), log.Id, err.Error())
		return
	}
}

// OnReqDetectedA 正向代理 让来自A的请求 转发给B
func (m *proxy) onReqA(pack PackReq) (EResp, any) {
	if m.mode == EProxyModeReverse || m.b == nil {
		return ERespBypass, nil
	}
	resp := m.b.Req(pack.To, pack.Route, pack.Content)
	if resp.RespCode != ERespSuccess {
		fmt.Printf("[%s]: %d %s %s(%d) %v\r\n", time.Now().Format("15:04:05.000"), resp.Id, "mqttProxy req A-> B Failed", getRespCodeName(resp.RespCode), resp.RespCode, resp.Content)
	}
	return resp.RespCode, resp.Content

}

// OnReqDetectedB 反向代理
func (m *proxy) onReqB(pack PackReq) (EResp, any) {
	if m.mode == EProxyModeForward || m.a == nil {
		return ERespBypass, nil
	}
	resp := m.a.Req(pack.To, pack.Route, pack.Content)
	return resp.RespCode, resp.Content

}
func getMonitorTopic(topic string) string {
	sections := strings.Split(topic, "/")
	sections = sections[:len(sections)-1]
	return strings.Join(sections, "/") + "/#"
}
