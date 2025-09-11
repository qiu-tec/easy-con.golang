/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/9/1 14:12
 */

package easyCon

import (
	"fmt"
	"strconv"
	"time"
)

type mqttProxy struct {
	a        IMonitor
	b        IMonitor
	mode     EProxyMode
	settingA ProxySetting
	settingB ProxySetting
	Id       string
}

func NewMqttProxy(settingA, settingB ProxySetting, mode EProxyMode) IProxy {
	proxy := &mqttProxy{mode: mode, settingA: settingA, settingB: settingB}
	proxy.Id = strconv.FormatInt(time.Now().UnixNano(), 10)
	sa := NewSetting("ProxyA"+proxy.Id, settingA.Addr, nil, nil)
	sa.LogMode = ELogModeNone
	sa.PreFix = settingA.PreFix
	sa.ReTry = settingA.ReTry
	sa.TimeOut = settingA.TimeOut
	sa.PWD = settingA.PWD
	sa.UID = settingA.UID
	sa.OnNotice = proxy.OnNoticeA
	sa.OnRetainNotice = proxy.OnRetainNoticeA
	sa.OnLog = proxy.OnLogA

	monitorSettingA := NewMonitorSetting(sa, settingA.ProxyModules, proxy.OnReqDetectedA, nil)
	monitorSettingA.OnDiscover = proxy.onDiscoverA

	sb := sa
	sb = NewSetting("ProxyB"+proxy.Id, settingB.Addr, nil, nil)
	sb.LogMode = ELogModeNone
	sb.PreFix = settingB.PreFix
	sb.ReTry = settingB.ReTry
	sb.TimeOut = settingB.TimeOut
	sb.PWD = settingB.PWD
	sb.UID = settingB.UID
	sb.OnNotice = proxy.OnNoticeB
	sb.OnRetainNotice = proxy.OnRetainNoticeB
	sb.OnLog = proxy.OnLogB
	monitorSettingB := NewMonitorSetting(sb, settingB.ProxyModules, proxy.OnReqDetectedB, nil)
	monitorSettingB.OnDiscover = proxy.onDiscoverB

	a := NewMqttMonitor(monitorSettingA)
	b := NewMqttMonitor(monitorSettingB)
	proxy.a = a
	proxy.b = b

	//if mode == EProxyModeReverse {
	//	a := NewMqttMonitor(monitorSettingA)
	//	b := NewMqttMonitor(monitorSettingB)
	//	proxy.a = a
	//	proxy.b = b
	//} else {
	//	b := NewMqttMonitor(monitorSettingB)
	//	a := NewMqttMonitor(monitorSettingA)
	//	proxy.a = a
	//	proxy.b = b
	//}

	return proxy
}

func (m *mqttProxy) Stop() {
	m.a.Stop()
	m.b.Stop()
}

func (m *mqttProxy) Reset() {
	m.a.Reset()
	m.b.Reset()
}

// OnNoticeA 收到来自A的通知
func (m *mqttProxy) OnNoticeA(notice PackNotice) {

	if m.mode == EProxyModeForward { //|| strings.HasSuffix(notice.From, "Proxy")
		return
	}
	for i := 0; i < m.settingB.ReTry; i++ {
		if m.b == nil {
			time.Sleep(m.settingB.TimeOut)
			continue
		}
		e := m.b.RelayNotice(notice)
		if e != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Notice A-> B Failed")
			time.Sleep(m.settingB.TimeOut)
			continue
		}
		return
	}
	fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Notice A-> B Failed")

}

func (m *mqttProxy) OnRetainNoticeA(notice PackNotice) {
	if m.mode == EProxyModeForward {
		return
	}

	for i := 0; i < m.settingB.ReTry; i++ {
		if m.b == nil {
			time.Sleep(m.settingB.TimeOut)
			continue
		}
		e := m.b.RelayRetainNotice(notice)
		if e != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Retain Notice A-> B Failed")
			time.Sleep(m.settingB.TimeOut)
			continue
		}
		return
	}
	fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Retain Notice A-> B outof retry")

}

func (m *mqttProxy) OnLogA(log PackLog) {
	if m.mode == EProxyModeForward {
		return
	}
	for i := 0; i < m.settingB.ReTry; i++ {
		if m.b == nil {
			time.Sleep(m.settingB.TimeOut)
			continue
		}
		e := m.b.RelayLog(log)
		if e != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Log A-> B Failed")
			time.Sleep(m.settingB.TimeOut)
			continue
		}
		return
	}
	fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Log A-> B outof retry")
}

func (m *mqttProxy) OnNoticeB(notice PackNotice) {

	if m.mode == EProxyModeReverse {
		return
	}
	for i := 0; i < m.settingA.ReTry; i++ {
		if m.a == nil {
			time.Sleep(m.settingB.TimeOut)
			continue
		}
		e := m.a.RelayNotice(notice)
		if e != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Notice B-> A Failed")
			time.Sleep(m.settingA.TimeOut)
			continue
		}
		return
	}
	fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Notice B-> A outof retry")
}

func (m *mqttProxy) OnRetainNoticeB(notice PackNotice) {
	if m.mode == EProxyModeReverse {
		return
	}
	for i := 0; i < m.settingA.ReTry; i++ {
		if m.a == nil {
			time.Sleep(m.settingB.TimeOut)
			continue
		}
		e := m.a.RelayRetainNotice(notice)
		if e != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Retain Notice B-> A failed")
			time.Sleep(m.settingA.TimeOut)
			continue
		}
		return
	}
	fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Retain Notice B-> A outof retry")
}

func (m *mqttProxy) OnLogB(log PackLog) {
	if m.mode == EProxyModeReverse {
		return
	}
	for i := 0; i < m.settingA.ReTry; i++ {
		if m.a == nil {
			time.Sleep(m.settingB.TimeOut)
			continue
		}
		e := m.a.RelayLog(log)
		if e != nil {
			fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Log B-> A Failed")
			time.Sleep(m.settingA.TimeOut)
			break
		}
		return
	}
	fmt.Printf("[%s]: %s \r\n", time.Now().Format("15:04:05.000"), "Log B-> A outof retry")
}

// OnReqDetectedA 正向代理 让来自A的请求 转发给B
func (m *mqttProxy) OnReqDetectedA(pack PackReq) {
	if m.mode == EProxyModeReverse {
		return
	}
	if m.b == nil {
		time.Sleep(m.settingB.TimeOut)
		m.OnReqDetectedA(pack)
	}
	resp := m.b.Req(pack.To, pack.Route, pack.Content)
	if resp.RespCode != ERespSuccess {
		fmt.Printf("[%s]: %d %s %d %s\r\n", time.Now().Format("15:04:05.000"), resp.Id, "proxy req A-> B Failed", resp.RespCode, resp.Content)
	}
	m.a.RelayResp(pack, resp.RespCode, resp.Content)

}

// OnReqDetectedB 反向代理
func (m *mqttProxy) OnReqDetectedB(pack PackReq) {
	if m.mode == EProxyModeForward {
		return
	}
	if m.a == nil {
		time.Sleep(m.settingB.TimeOut)
		m.OnReqDetectedA(pack)
	}

	resp := m.a.Req(pack.To, pack.Route, pack.Content)
	if resp.RespCode != ERespSuccess {
		fmt.Printf("[%s]: %d %s %s %s\r\n", time.Now().Format("15:04:05.000"), resp.Id, "proxy req B-> A Failed", resp.RespCode, resp.Content)
	}
	m.b.RelayResp(pack, resp.RespCode, resp.Content)

}

func (m *mqttProxy) onDiscoverA(module string) {
	if m.mode == EProxyModeForward || m.settingA.ProxyModules != nil || module == "ProxyA"+m.Id { //正向代理时不需要处理 已经设置好代理时不需要处理
		return
	}
	if m.b == nil {
		time.Sleep(m.settingB.TimeOut)
		m.onDiscoverA(module)
	}
	m.b.Discover(module)
}

func (m *mqttProxy) onDiscoverB(module string) {

	if m.mode == EProxyModeReverse || m.settingB.ProxyModules != nil || module == "ProxyB"+m.Id { //反向代理时不需要处理 已经设置好代理时不需要处理
		return
	}
	if m.a == nil {
		time.Sleep(m.settingB.TimeOut)
		m.onDiscoverB(module)
	}
	m.a.Discover(module)
}
