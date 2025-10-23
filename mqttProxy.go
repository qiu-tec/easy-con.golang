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

// retryWithBackoff 执行重试逻辑
func (m *mqttProxy) retryWithBackoff(operation func() error, retries int, timeout time.Duration, operationName string) bool {
	for i := 0; i < retries; i++ {
		if err := operation(); err != nil {
			fmt.Printf("[%s]: %s failed (attempt %d/%d): %v\r\n",
				time.Now().Format("15:04:05.000"), operationName, i+1, retries, err)
			if i < retries-1 {
				time.Sleep(timeout)
			}
			continue
		}
		return true
	}
	fmt.Printf("[%s]: %s failed after %d attempts\r\n",
		time.Now().Format("15:04:05.000"), operationName, retries)
	return false
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

	sb := NewSetting("ProxyB"+proxy.Id, settingB.Addr, nil, nil)
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
	if m.mode == EProxyModeForward || !m.settingA.ProxyNotice || m.b == nil {
		return
	}

	m.retryWithBackoff(func() error {
		return m.b.RelayNotice(notice)
	}, m.settingA.ReTry, m.settingB.TimeOut, "Notice A-> B")
}

func (m *mqttProxy) OnRetainNoticeA(notice PackNotice) {
	if m.mode == EProxyModeForward || !m.settingA.ProxyRetainNotice || m.b == nil {
		return
	}

	m.retryWithBackoff(func() error {
		return m.b.RelayRetainNotice(notice)
	}, m.settingA.ReTry, m.settingB.TimeOut, "Retain Notice A-> B")
}

func (m *mqttProxy) OnLogA(log PackLog) {
	if m.mode == EProxyModeForward || !m.settingA.ProxyLog || m.b == nil {
		return
	}

	m.retryWithBackoff(func() error {
		return m.b.RelayLog(log)
	}, m.settingA.ReTry, m.settingB.TimeOut, "Log A-> B")
}

func (m *mqttProxy) OnNoticeB(notice PackNotice) {
	if m.mode == EProxyModeReverse || !m.settingB.ProxyNotice || m.a == nil {
		return
	}

	m.retryWithBackoff(func() error {
		return m.a.RelayNotice(notice)
	}, m.settingB.ReTry, m.settingA.TimeOut, "Notice B-> A")
}

func (m *mqttProxy) OnRetainNoticeB(notice PackNotice) {
	if m.mode == EProxyModeReverse || !m.settingB.ProxyRetainNotice || m.a == nil {
		return
	}

	m.retryWithBackoff(func() error {
		return m.a.RelayRetainNotice(notice)
	}, m.settingB.ReTry, m.settingA.TimeOut, "Retain Notice B-> A")
}

func (m *mqttProxy) OnLogB(log PackLog) {
	if m.mode == EProxyModeReverse || !m.settingB.ProxyLog || m.a == nil {
		return
	}

	m.retryWithBackoff(func() error {
		return m.a.RelayLog(log)
	}, m.settingB.ReTry, m.settingA.TimeOut, "Log B-> A")
}

// OnReqDetectedA 正向代理 让来自A的请求 转发给B
func (m *mqttProxy) OnReqDetectedA(pack PackReq) {
	if m.mode == EProxyModeReverse {
		return
	}
	if m.b == nil {
		time.Sleep(m.settingB.TimeOut)
		return // 修复：直接返回而不是递归调用
	}
	resp := m.b.Req(pack.To, pack.Route, pack.Content)
	if resp.RespCode != ERespSuccess {
		fmt.Printf("[%s]: %d %s %s(%d) %v\r\n", time.Now().Format("15:04:05.000"), resp.Id, "proxy req A-> B Failed", getRespCodeName(resp.RespCode), resp.RespCode, resp.Content)
	}
	m.a.RelayResp(pack, resp.RespCode, resp.Content)

}

// OnReqDetectedB 反向代理
func (m *mqttProxy) OnReqDetectedB(pack PackReq) {
	if m.mode == EProxyModeForward {
		return
	}
	if m.a == nil {
		time.Sleep(m.settingA.TimeOut) // 修复：使用settingA的超时时间
		return                         // 修复：直接返回而不是错误的递归调用
	}

	resp := m.a.Req(pack.To, pack.Route, pack.Content)
	if resp.RespCode != ERespSuccess {
		fmt.Printf("[%s]: %d %s %s(%d) %v\r\n", time.Now().Format("15:04:05.000"), resp.Id, "proxy req B-> A Failed", getRespCodeName(resp.RespCode), resp.RespCode, resp.Content)
	}
	m.b.RelayResp(pack, resp.RespCode, resp.Content)

}

func (m *mqttProxy) onDiscoverA(module string) {
	if m.mode == EProxyModeForward || m.settingA.ProxyModules != nil || module == "ProxyA"+m.Id { //正向代理时不需要处理 已经设置好代理时不需要处理
		return
	}
	if m.b == nil {
		time.Sleep(m.settingB.TimeOut)
		return // 修复：避免递归调用
	}
	m.b.Discover(module)
}

func (m *mqttProxy) onDiscoverB(module string) {

	if m.mode == EProxyModeReverse || m.settingB.ProxyModules != nil || module == "ProxyB"+m.Id { //反向代理时不需要处理 已经设置好代理时不需要处理
		return
	}
	if m.a == nil {
		time.Sleep(m.settingA.TimeOut) // 修复：使用settingA的超时时间
		return                         // 修复：避免递归调用
	}
	m.a.Discover(module)
}
