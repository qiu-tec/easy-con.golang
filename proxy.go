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
	notice.From = p.sb.Module + "/" + notice.From
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
	log.From = p.sb.Module + "/" + log.From

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
	notice.From = p.sa.Module + "/" + notice.From
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
	//if p.mode == EProxyModeReverse || !p.proxyRetainNotice || p.a == nil {
	//	return
	//}
	if strings.HasPrefix(notice.From, p.sb.Module) { //来自自己
		return
	}
	notice.From = p.sb.Module + "/" + notice.From
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
	//if p.mode == EProxyModeReverse || !p.proxyLog || p.a == nil {
	//	return
	//}
	if strings.HasPrefix(log.From, p.sb.Module) { //来自自己
		return
	}
	log.From = p.sa.Module + "/" + log.From
	topic := BuildLogTopic(p.sa.PreFix)

	rawData, err := log.Raw()
	if err != nil {
		fmt.Printf("[%s]:mqttProxy log raw failed: %s\r\n",
			time.Now().Format("15:04:05.000"), err.Error())
		return
	}

	err = p.a.PublishRaw(topic, false, rawData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy log (%d) B->A failed: %s\r\n",
			time.Now().Format("15:04:05.000"), log.Id, err.Error())
	}
}

// OnReqDetectedA 正向代理 让来自A的请求 转发给B
func (p *proxy) onReqA(pack PackReq) (EResp, []byte) {
	if pack.To == "Broker" {
		return ERespBypass, []byte{}
	}
	if strings.HasPrefix(pack.From, p.sa.Module) { //来自自己
		return ERespBypass, []byte{}
	}

	// 直接转发原始字节，零拷贝
	topic := BuildReqTopic(p.sb.PreFix, pack.To)

	// 获取原始消息
	rawData, err := pack.Raw()
	if err != nil {
		fmt.Printf("[%s]:mqttProxy pack raw failed: %s\r\n",
			time.Now().Format("15:04:05.000"), err.Error())
		return ERespError, []byte{}
	}

	// 直接发布原始字节
	err = p.b.PublishRaw(topic, false, rawData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy req (%d) A->B failed: %s\r\n",
			time.Now().Format("15:04:05.000"), pack.Id, err.Error())
		return ERespError, []byte{}
	}

	return ERespBypass, []byte{}
}

// OnReqDetectedB 反向代理
func (p *proxy) onReqB(pack PackReq) (EResp, []byte) {

	if strings.HasPrefix(pack.From, p.sb.Module) { //来自自己
		return ERespBypass, []byte{}
	}

	// 直接转发原始字节，零拷贝
	topic := BuildReqTopic(p.sa.PreFix, pack.To)

	// 获取原始消息
	rawData, err := pack.Raw()
	if err != nil {
		fmt.Printf("[%s]:mqttProxy pack raw failed: %s\r\n",
			time.Now().Format("15:04:05.000"), err.Error())
		return ERespError, []byte{}
	}

	// 直接发布原始字节
	err = p.a.PublishRaw(topic, false, rawData)
	if err != nil {
		fmt.Printf("[%s]:mqttProxy req (%d) B->A failed: %s\r\n",
			time.Now().Format("15:04:05.000"), pack.Id, err.Error())
		return ERespError, []byte{}
	}

	return ERespBypass, []byte{}
}

func getMonitorTopic(topic string) string {
	sections := strings.Split(topic, "/")
	sections = sections[:len(sections)-1]
	return strings.Join(sections, "/") + "/#"
}
