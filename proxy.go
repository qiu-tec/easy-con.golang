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

	sa CoreSetting
	sb CoreSetting
	//mode              EProxyMode
	proxyModules      []string
	proxyNotice       bool
	proxyRetainNotice bool
	proxyLog          bool
}

func NewCgoMqttProxy(setting MqttProxySetting, onWrite func([]byte) error) (IProxy, func([]byte)) {

	p := &proxy{
		//mode:              EProxyModeForward,
		proxyModules:      nil,
		proxyNotice:       true,
		proxyRetainNotice: true,
		proxyLog:          true,
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
	a, f := NewCGoMonitor(sa, AdapterCallBack{
		OnReqRec:          p.onReqA,
		OnNoticeRec:       p.onNoticeA,
		OnRetainNoticeRec: p.onRetainNoticeA,
		OnLogRec:          p.onLogA,
		//OnRespRec:         p.onRespA,
	}, onWrite)
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
		//OnRespRec:         p.onRespB,
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
func (p *proxy) onNoticeA(notice PackNotice) {
	if strings.HasPrefix(notice.From, p.sa.Module) { //来自自己
		return
	}
	notice.From = p.sb.Module + "/" + notice.From
	topic := BuildNoticeTopic(p.sb.PreFix, notice.Route)
	err := p.b.Publish(topic, false, &notice)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy notice  (%d) route %s  A-> B Failed because %s\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Route, err.Error())
		return
	}

}

func (p *proxy) onRetainNoticeA(notice PackNotice) {
	if strings.HasPrefix(notice.From, p.sa.Module) { //来自自己
		return
	}
	notice.From = p.sb.Module + "/" + notice.From
	topic := BuildRetainNoticeTopic(p.sb.PreFix, notice.Route)
	err := p.b.Publish(topic, true, &notice)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy retain notice  (%d) route %s  A-> B Failed because %s\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Route, err.Error())
		return
	}
}

func (p *proxy) onLogA(log PackLog) {
	if strings.HasPrefix(log.From, p.sa.Module) { //来自自己
		return
	}
	log.From = p.sb.Module + "/" + log.From

	topic := BuildLogTopic(p.sb.PreFix)
	err := p.b.Publish(topic, false, &log)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy log  (%d) A-> B Failed because %s\r\n", time.Now().Format("15:04:05.000"), log.Id, err.Error())
		return
	}
}

func (p *proxy) onNoticeB(notice PackNotice) {
	if strings.HasPrefix(notice.From, p.sb.Module) { //来自自己
		return
	}
	notice.From = p.sa.Module + "/" + notice.From
	topic := BuildNoticeTopic(p.sa.PreFix, notice.Route)
	err := p.a.Publish(topic, false, &notice)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy notice  (%d) route %s  B-> A Failed because %s\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Route, err.Error())
		return
	}
}

func (p *proxy) onRetainNoticeB(notice PackNotice) {
	//if p.mode == EProxyModeReverse || !p.proxyRetainNotice || p.a == nil {
	//	return
	//}
	if strings.HasPrefix(notice.From, p.sb.Module) { //来自自己
		return
	}
	notice.From = p.sb.Module + "/" + notice.From
	topic := BuildRetainNoticeTopic(p.sa.PreFix, notice.Route)
	err := p.a.Publish(topic, true, &notice)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy retain notice  (%d) route %s  B-> A Failed because %s\r\n", time.Now().Format("15:04:05.000"), notice.Id, notice.Route, err.Error())
		return
	}
}

func (p *proxy) onLogB(log PackLog) {
	//if p.mode == EProxyModeReverse || !p.proxyLog || p.a == nil {
	//	return
	//}
	if strings.HasPrefix(log.From, p.sb.Module) { //来自自己
		return
	}
	log.From = p.sa.Module + "/" + log.From
	topic := BuildLogTopic(p.sa.PreFix)
	err := p.a.Publish(topic, false, &log)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy log  (%d)  B-> A Failed because %s\r\n", time.Now().Format("15:04:05.000"), log.Id, err.Error())
		return
	}
}

// OnReqDetectedA 正向代理 让来自A的请求 转发给B
func (p *proxy) onReqA(pack PackReq) (EResp, []byte) {
	if pack.To == "Broker" {
		return ERespBypass, nil
	}
	if strings.HasPrefix(pack.From, p.sa.Module) { //来自自己
		return ERespBypass, nil
	}
	resp := p.b.Req(pack.To, pack.Route, pack.Content)
	topic := BuildRespTopic(p.sa.PreFix, pack.From)
	respPack := newRespPack(pack, resp.RespCode, resp.Content)
	err := p.a.Publish(topic, false, &respPack)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy resp  (%d) A-> B Failed because %s\r\n", time.Now().Format("15:04:05.000"), pack.Id, err.Error())
	}
	return ERespBypass, nil
}

// OnReqDetectedB 反向代理
func (p *proxy) onReqB(pack PackReq) (EResp, []byte) {

	if strings.HasPrefix(pack.From, p.sb.Module) { //来自自己
		return ERespBypass, nil
	}

	resp := p.a.Req(pack.To, pack.Route, pack.Content)
	topic := BuildRespTopic(p.sb.PreFix, pack.From)
	respPack := newRespPack(pack, resp.RespCode, resp.Content)
	err := p.b.Publish(topic, false, &respPack)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy resp  (%d) A-> B Failed because %s\r\n", time.Now().Format("15:04:05.000"), pack.Id, err.Error())
	}
	return ERespBypass, nil
}

func getMonitorTopic(topic string) string {
	sections := strings.Split(topic, "/")
	sections = sections[:len(sections)-1]
	return strings.Join(sections, "/") + "/#"
}
