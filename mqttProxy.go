/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/9/1 14:12
 */

package easyCon

type mqttProxy struct {
	a        IMonitor
	b        IMonitor
	mode     EProxyMode
	settingA ProxySetting
	settingB ProxySetting
}

func NewMqttProxy(settingA, settingB ProxySetting, mode EProxyMode) IProxy {
	proxy := &mqttProxy{mode: mode, settingA: settingA, settingB: settingB}

	sa := NewSetting(settingA.Module, settingA.Addr, nil, nil)
	sa.LogMode = settingA.LogMode
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
	sb = NewSetting(settingB.Module, settingB.Addr, nil, nil)
	sb.LogMode = settingB.LogMode
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
	if mode == EProxyModeReverse {
		a := NewMqttMonitor(monitorSettingA)
		b := NewMqttMonitor(monitorSettingB)
		proxy.a = a
		proxy.b = b
	} else {
		b := NewMqttMonitor(monitorSettingB)
		a := NewMqttMonitor(monitorSettingA)
		proxy.a = a
		proxy.b = b
	}

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
	m.b.RelayNotice(notice)

}

func (m *mqttProxy) OnRetainNoticeA(notice PackNotice) {
	if m.mode == EProxyModeForward {
		return
	}
	m.b.RelayRetainNotice(notice)

}

func (m *mqttProxy) OnLogA(log PackLog) {
	if m.mode == EProxyModeForward {
		return
	}
	m.b.RelayLog(log)
}

func (m *mqttProxy) OnNoticeB(notice PackNotice) {

	if m.mode == EProxyModeReverse {
		return
	}
	m.a.RelayNotice(notice)

}

func (m *mqttProxy) OnRetainNoticeB(notice PackNotice) {
	if m.mode == EProxyModeReverse {
		return
	}
	m.a.RelayRetainNotice(notice)
}

func (m *mqttProxy) OnLogB(log PackLog) {
	if m.mode == EProxyModeReverse {
		return
	}
	m.a.RelayLog(log)
}

// OnReqDetectedA 正向代理 让来自A的请求 转发给B
func (m *mqttProxy) OnReqDetectedA(pack PackReq) {
	if m.mode == EProxyModeReverse {
		return
	}
	//go func() {
	resp := m.b.Req(pack.To, pack.Route, pack.Content)
	m.a.RelayResp(pack, resp.RespCode, resp.Content)
	//}()

}

// OnReqDetectedB 反向代理
func (m *mqttProxy) OnReqDetectedB(pack PackReq) {
	if m.mode == EProxyModeForward {
		return
	}
	//go func() {
	resp := m.a.Req(pack.To, pack.Route, pack.Content)
	m.b.RelayResp(pack, resp.RespCode, resp.Content)
	//}()

}

func (m *mqttProxy) onDiscoverA(module string) {
	if m.mode == EProxyModeForward || m.settingA.ProxyModules != nil || module == m.settingA.Module { //正向代理时不需要处理 已经设置好代理时不需要处理
		return
	}
	m.b.Discover(module)
}

func (m *mqttProxy) onDiscoverB(module string) {

	if m.mode == EProxyModeReverse || m.settingB.ProxyModules != nil || module == m.settingB.Module { //反向代理时不需要处理 已经设置好代理时不需要处理
		return
	}
	m.a.Discover(module)
}
