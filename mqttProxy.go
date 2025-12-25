/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/9/1 14:12
 */

package easyCon

func NewMqttProxy(settingA, settingB MqttProxySetting, mode EProxyMode, proxyNotice, ProxyRetainNotice, ProxyLog bool, ProxyModules []string) IProxy {
	p := &proxy{
		mode:              mode,
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
		OnRetainNoticeRec: p.onRetainNoticeA,
		OnNoticeRec:       p.onNoticeA,
		OnLogRec:          p.onLogA,
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
		OnRetainNoticeRec: p.onRetainNoticeB,
		OnNoticeRec:       p.onNoticeB,
		OnLogRec:          p.onLogB,
	}
	//fmt.Println(cbb)
	a := NewMqttMonitor(sa, cba)
	b := NewMqttMonitor(sb, cbb)
	p.a = a
	p.b = b

	return p
}
