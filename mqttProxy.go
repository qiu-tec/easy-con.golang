/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/9/1 14:12
 */

package easyCon

import (
	"fmt"
	"time"
)

type proxy struct {
	a        IAdapter
	b        IAdapter
	mode     EProxyMode
	settingA MqttProxySetting
	settingB MqttProxySetting

	proxyModules      []string
	proxyNotice       bool
	proxyRetainNotice bool
	proxyLog          bool
}

func NewMqttProxy(settingA, settingB MqttProxySetting, mode EProxyMode, proxyNotice, ProxyRetainNotice, ProxyLog bool, ProxyModules []string) IProxy {
	p := &proxy{
		mode:              mode,
		settingA:          settingA,
		settingB:          settingB,
		proxyModules:      ProxyModules,
		proxyNotice:       proxyNotice,
		proxyRetainNotice: ProxyRetainNotice,
		proxyLog:          ProxyLog,
	}

	sa := NewDefaultMqttSetting("Proxy", settingA.Addr)
	sa.IsRandomClientID = true
	sa.LogMode = ELogModeNone
	sa.PreFix = settingA.PreFix
	sa.ReTry = settingA.ReTry
	sa.TimeOut = settingA.TimeOut
	sa.PWD = settingA.PWD
	sa.UID = settingA.UID

	cba := AdapterCallBack{
		OnReqRec:          p.onReqA,
		OnRetainNoticeRec: p.OnRetainNoticeA,
		OnNoticeRec:       p.OnNoticeA,
		OnLogRec:          p.OnLogA,
	}
	sb := NewDefaultMqttSetting("Proxy", settingB.Addr)
	sb.LogMode = ELogModeNone
	sb.PreFix = settingB.PreFix
	sb.ReTry = settingB.ReTry
	sb.TimeOut = settingB.TimeOut
	sb.PWD = settingB.PWD
	sb.UID = settingB.UID

	cbb := AdapterCallBack{
		OnReqRec:          p.onReqB,
		OnRetainNoticeRec: p.OnRetainNoticeB,
		OnNoticeRec:       p.OnNoticeB,
		OnLogRec:          p.OnLogB,
	}
	//fmt.Println(cbb)
	a := NewMqttMonitor(sa, cba)
	b := NewMqttMonitor(sb, cbb)
	p.a = a
	p.b = b

	return p
}

func (m *proxy) Stop() {
	m.a.Stop()
	m.b.Stop()
}

func (m *proxy) Reset() {
	m.a.Reset()
	m.b.Reset()
}

//// retryWithBackoff 执行重试逻辑
//func (m *proxy) retryWithBackoff(operation func() error, retries int, timeout time.Duration, operationName string) bool {
//	for i := 0; i < retries; i++ {
//		if err := operation(); err != nil {
//			fmt.Printf("[%s]: %s failed (attempt %d/%d): %v\r\n",
//				time.Now().Format("15:04:05.000"), operationName, i+1, retries, err)
//			if i < retries-1 {
//				time.Sleep(timeout)
//			}
//			continue
//		}
//		return true
//	}
//	fmt.Printf("[%s]: %s failed after %d attempts\r\n",
//		time.Now().Format("15:04:05.000"), operationName, retries)
//	return false
//}

// OnNoticeA 收到来自A的通知
func (m *proxy) OnNoticeA(notice PackNotice) {
	if m.mode == EProxyModeForward || !m.proxyNotice || m.b == nil {
		return
	}

	topic := BuildNoticeTopic(m.settingB.PreFix, notice.Route)
	err := m.b.Publish(topic, false, &notice)
	if err != nil {
		fmt.Printf("[%s]:proxy notice  (%d) route %s  A-> B Failed because %s\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Route, err.Error())
		return
	}

}

func (m *proxy) OnRetainNoticeA(notice PackNotice) {
	if m.mode == EProxyModeForward || !m.proxyRetainNotice || m.b == nil {
		return
	}
	topic := BuildRetainNoticeTopic(m.settingB.PreFix, notice.Route)
	err := m.b.Publish(topic, true, &notice)
	if err != nil {
		fmt.Printf("[%s]:proxy retain notice  (%d) route %s  A-> B Failed because %s\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Route, err.Error())
		return
	}
}

func (m *proxy) OnLogA(log PackLog) {
	if m.mode == EProxyModeForward || !m.proxyLog || m.b == nil {
		return
	}
	topic := BuildLogTopic(m.settingB.PreFix)
	err := m.b.Publish(topic, false, &log)
	if err != nil {
		fmt.Printf("[%s]:proxy log  (%d) A-> B Failed because %s\r\n", time.Now().Format("15:04:05.000"), log.Id, err.Error())
		return
	}
}

func (m *proxy) OnNoticeB(notice PackNotice) {
	if m.mode == EProxyModeReverse || !m.proxyNotice || m.a == nil {
		return
	}
	topic := BuildNoticeTopic(m.settingA.PreFix, notice.Route)
	err := m.a.Publish(topic, false, &notice)
	if err != nil {
		fmt.Printf("[%s]:proxy notice  (%d) route %s  B-> A Failed because %s\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Route, err.Error())
		return
	}
}

func (m *proxy) OnRetainNoticeB(notice PackNotice) {
	if m.mode == EProxyModeReverse || !m.proxyRetainNotice || m.a == nil {
		return
	}
	topic := BuildRetainNoticeTopic(m.settingA.PreFix, notice.Route)
	err := m.a.Publish(topic, true, &notice)
	if err != nil {
		fmt.Printf("[%s]:proxy retain notice  (%d) route %s  B-> A Failed because %s\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Route, err.Error())
		return
	}
}

func (m *proxy) OnLogB(log PackLog) {
	if m.mode == EProxyModeReverse || !m.proxyLog || m.a == nil {
		return
	}
	topic := BuildLogTopic(m.settingA.PreFix)
	err := m.a.Publish(topic, false, &log)
	if err != nil {
		fmt.Printf("[%s]:proxy log  (%d)  B-> A Failed because %s\r\n", time.Now().Format("15:04:05.000"), log.Id, err.Error())
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
		fmt.Printf("[%s]: %d %s %s(%d) %v\r\n", time.Now().Format("15:04:05.000"), resp.Id, "proxy req A-> B Failed", getRespCodeName(resp.RespCode), resp.RespCode, resp.Content)
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
