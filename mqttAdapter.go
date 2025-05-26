/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2024/7/11 11:52
 */

package easyCon

import (
	"encoding/json"
	"errors"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"sync"
	"time"
)

// mqttAdapter MQTT访问器 模块
type mqttAdapter struct {
	client   mqtt.Client
	setting  Setting
	respDict map[uint64]chan PackResp
	mu       sync.RWMutex
	//请求chan
	reqChan chan mqtt.Message
	//响应chan
	respChan chan mqtt.Message
	stopChan chan struct{}
	logChan  chan PackLog
	options  *mqtt.ClientOptions
	wg       *sync.WaitGroup
	isLinked bool
}

func NewMqttAdapter(setting Setting) IAdapter {
	return newMqttAdapter(setting)
}
func newMqttAdapter(setting Setting) *mqttAdapter {
	return newMqttAdapterWithAfterLink(setting, nil)
}

func newMqttAdapterWithAfterLink(setting Setting, afterLink func(client mqtt.Client)) *mqttAdapter {
	adapter := &mqttAdapter{
		respDict: make(map[uint64]chan PackResp),
		mu:       sync.RWMutex{},
		reqChan:  make(chan mqtt.Message, 100),
		respChan: make(chan mqtt.Message, 100),

		stopChan: make(chan struct{}),
		logChan:  make(chan PackLog, 100),

		wg: &sync.WaitGroup{},
	}
	adapter.setting = setting
	o := mqtt.NewClientOptions().
		SetClientID(setting.Module).
		AddBroker(setting.Addr).
		SetUsername(setting.UID).
		SetPassword(setting.PWD).
		SetAutoReconnect(true)
	o.OnConnect = func(client mqtt.Client) {
		adapter.isLinked = true
		adapter.setting.StatusChanged(EStatusLinked)

		//订阅请求主题
		topic := buildReqTopic(adapter.setting.Module)
		token := adapter.client.Subscribe(topic, 0, func(_ mqtt.Client, message mqtt.Message) {
			adapter.reqChan <- message
		})
		if token.Wait() && token.Error() != nil {
			adapter.Err("Req Subscribe error", token.Error())
		}
		adapter.Debug(topic + " Subscribed")
		//订阅响应主题
		topic = buildRespTopic(adapter.setting.Module)
		token = adapter.client.Subscribe(topic, 0, func(_ mqtt.Client, message mqtt.Message) {
			adapter.respChan <- message
		})
		if token.Wait() && token.Error() != nil {
			adapter.Err("Resp Subscribe error", token.Error())
		}
		adapter.Debug(topic + " Subscribed")
		//如果通知回调不为空，订阅通知主题
		if adapter.setting.OnNotice != nil {
			topic = adapter.setting.PreFix + NoticeTopic
			token = adapter.client.Subscribe(topic, 0, func(_ mqtt.Client, message mqtt.Message) {
				notice := &PackNotice{}
				err := json.Unmarshal(message.Payload(), notice)
				if err != nil {
					adapter.Err("Notice Unmarshal error", err)
				}
				adapter.setting.OnNotice(*notice)
			})
			if token.Wait() && token.Error() != nil {
				adapter.Err("Notice Subscribe error", token.Error())
			}
			adapter.Debug(topic + " Subscribed")

		}
		if adapter.setting.OnRetainNotice != nil {
			topic = adapter.setting.PreFix + RetainNoticeTopic
			token = adapter.client.Subscribe(topic, 0, func(_ mqtt.Client, message mqtt.Message) {
				notice := &PackNotice{}
				err := json.Unmarshal(message.Payload(), notice)
				if err != nil {
					adapter.Err("Notice Unmarshal error", err)
				}
				adapter.setting.OnRetainNotice(*notice)
			})
			if token.Wait() && token.Error() != nil {
				adapter.Err("RetainNotice Subscribe error", token.Error())
			}
			adapter.Debug(topic + " Subscribed")
		}
		//如果日志回调不为空，订阅日志主题
		if adapter.setting.OnLog != nil {
			topic = adapter.setting.PreFix + LogTopic
			token = adapter.client.Subscribe(topic, 0, func(_ mqtt.Client, message mqtt.Message) {
				log := &PackLog{}
				err := json.Unmarshal(message.Payload(), log)
				if err != nil {
					adapter.Err("Log Unmarshal error", err)
				}
				adapter.setting.OnLog(*log)
			})
			if token.Wait() && token.Error() != nil {
				adapter.Err("Log Subscribe error", token.Error())
			}
			adapter.Debug(topic + " Subscribed")
		}
		if afterLink != nil {
			afterLink(client)
		}
	}

	o.OnConnectionLost = func(client mqtt.Client, err error) {
		adapter.isLinked = false
		adapter.setting.StatusChanged(EStatusLinkLost)
	}
	o.OnReconnecting = func(client mqtt.Client, options *mqtt.ClientOptions) {
		adapter.setting.StatusChanged(EStatusConnecting)
	}
	adapter.options = o
	adapter.link()
	return adapter
}

func (adapter *mqttAdapter) Stop() {
	go func() {
		//time.Sleep(10)
		adapter.stopChan <- struct{}{}
	}()
	adapter.wg.Wait()
	adapter.client.Disconnect(10)
	adapter.setting.StatusChanged(EStatusStopped)
}

func (adapter *mqttAdapter) Reset() {
	adapter.Stop()
	adapter.link()

}

func (adapter *mqttAdapter) Req(module, route string, params any) PackResp {
	if !adapter.isLinked {
		return PackResp{
			RespCode: ERespUnLinked,
		}
	}
	pack := newReqPack(adapter.setting.Module, module, route, params)
	for retry := adapter.setting.ReTry; retry >= 0; retry-- {
		resp := adapter.req(pack)
		switch resp.RespCode {
		case ERespTimeout:
			continue
		default:
			return resp
		}
	}
	p := PackResp{
		PackReq:  pack,
		RespTime: "",
		RespCode: ERespTimeout,
		Error:    "Req timeout",
	}
	p.Content = nil
	return p
}

func (adapter *mqttAdapter) SendRetainNotice(route string, content any) error {
	return adapter.sendNotice(route, true, content)
}

func (adapter *mqttAdapter) SendNotice(route string, content any) error {
	return adapter.sendNotice(route, false, content)
}
func (adapter *mqttAdapter) sendNotice(route string, isRetain bool, content any) error {
	if !adapter.isLinked {
		return errors.New("adapter is not linked")
	}
	pack := newNoticePack(adapter.setting.Module, route, content)
	js, err := json.Marshal(pack)
	if err != nil {
		adapter.Err("Notice marshal error", err)
		return err
	}
	topic := adapter.setting.PreFix + NoticeTopic
	if isRetain {
		topic = RetainNoticeTopic
	}
	token := adapter.client.Publish(topic, 0, isRetain, js)
	if token.Wait() && token.Error() != nil {
		adapter.Err("Notice send error", token.Error())
		return err
	}
	return nil
}
func (adapter *mqttAdapter) Debug(content string) {
	pack := newLogPack(adapter.setting.Module, ELogLevelDebug, content, nil)
	adapter.logChan <- pack
}

func (adapter *mqttAdapter) Warn(content string) {
	pack := newLogPack(adapter.setting.Module, ELogLevelWarning, content, nil)
	adapter.logChan <- pack
}

func (adapter *mqttAdapter) Err(content string, err error) {
	pack := newLogPack(adapter.setting.Module, ELogLevelError, content, err)
	adapter.logChan <- pack
}

func (adapter *mqttAdapter) req(pack PackReq) PackResp {
	raw, err := json.Marshal(pack)
	if err != nil {
		adapter.Err(fmt.Sprintf("REQ %d json error\r\n", pack.Id), err)
		return PackResp{
			RespCode: ERespBadReq,
			Error:    err.Error(),
		}
	}

	token := adapter.client.Publish(buildReqTopic(pack.To), 0, false, raw)
	if token.Wait() && token.Error() != nil {
		adapter.Err(fmt.Sprintf("REQ %d send error\r\n", pack.Id), token.Error())
		return PackResp{
			RespCode: ERespUnLinked,
		}
	}
	respChan := make(chan PackResp)
	adapter.mu.Lock()
	adapter.respDict[pack.Id] = respChan
	adapter.mu.Unlock()
	select {
	case resp := <-respChan:
		adapter.mu.Lock()
		delete(adapter.respDict, pack.Id)
		adapter.mu.Unlock()
		return resp
	case <-time.After(adapter.setting.TimeOut):
		adapter.mu.Lock()
		delete(adapter.respDict, pack.Id)
		adapter.mu.Unlock()
		return PackResp{
			RespCode: ERespTimeout,
		}
	}
}

func (adapter *mqttAdapter) onResp(message mqtt.Message) {

	pack := &PackResp{}
	err := json.Unmarshal(message.Payload(), pack)
	if err != nil {
		adapter.Err("RESP unmarshal error", err)
	}
	adapter.Debug("Resp received raw is: " + string(message.Payload()))
	if pack.RespCode != ERespSuccess && pack.Content != nil {
		pack.Error = pack.Content.(error).Error()
		pack.Content = nil
	}
	adapter.mu.RLock()
	c, b := adapter.respDict[pack.Id]
	if b {
		c <- *pack
	}
	adapter.mu.RUnlock()
}

func (adapter *mqttAdapter) onReq(message mqtt.Message) {
	pack := &PackReq{}
	var respPack PackResp
	defer func() {
		js, err := json.Marshal(respPack)
		if err != nil {
			adapter.Err("RESP marshal error", err)
			return
		}
		token := adapter.client.Publish(buildRespTopic(pack.From), 0, false, js)
		if token.Wait() && token.Error() != nil {
			adapter.Err("RESP send error", token.Error())
			return
		}
		//adapter.detectedChan <- keyValuePair{pack.Route, js}
	}()

	err := json.Unmarshal(message.Payload(), pack)
	if err != nil {
		respPack = newRespPack(*pack, ERespBadReq, err)
		//respPack.RespCode = ERespBadReq
		//respPack.Error = err.Error()
		return
	}
	adapter.Debug("Req Received raw is: " + string(message.Payload()))
	resp, content := adapter.setting.OnReq(*pack)
	respPack = newRespPack(*pack, resp, content)

}

//// checkRoute 检查路由是否匹配
//func checkRoute(route string, routes []string) bool {
//	for _, r := range routes {
//		//完全匹配
//		if r == route {
//			return true
//		}
//		//正则匹配
//		if regexp.MustCompile(r).MatchString(route) {
//			return true
//		}
//	}
//	return false
//}

func (adapter *mqttAdapter) loop() {
	adapter.wg.Add(1)
	defer adapter.wg.Done()
	for {
		select {
		case msg := <-adapter.reqChan:
			go adapter.onReq(msg)
		case msg := <-adapter.respChan:
			go adapter.onResp(msg)
		//case msg := <-adapter.detectedChan:
		//	go adapter.onDetected(msg.Key, msg.Value.([]byte))
		//case msg := <-adapter.watchedChan:
		//	go adapter.setting.OnRespDetected(msg)
		case pack := <-adapter.logChan:
			go adapter.sendLog(pack)
		case <-adapter.stopChan:
			return
		}
	}
}

func (adapter *mqttAdapter) link() {
	go adapter.loop()
	adapter.client = mqtt.NewClient(adapter.options)
ReLink:
	token := adapter.client.Connect()
	if token.Wait() && token.Error() != nil {
		goto ReLink
	}
}

func (adapter *mqttAdapter) sendLog(pack PackLog) {
	if adapter.setting.LogMode == ELogModeConsole || adapter.setting.LogMode == ELogModeAll {
		printLog(pack)
	}
	if adapter.setting.LogMode == ELogModeUpload || adapter.setting.LogMode == ELogModeAll {

		if adapter.isLinked == false { //未连接 就不再继续尝试发包了
			return
		}
		js, err := json.Marshal(pack)
		if err != nil {
			fmt.Printf("[%s][%s][Error]: %s\r\n", time.Now().Format("2006-01-02 15:04:05.000"), adapter.setting.Module, err.Error())
			return
		}
		token := adapter.client.Publish(adapter.setting.PreFix+LogTopic, 0, false, js)
		if token.Wait() && token.Error() != nil {
			adapter.Err("Log send error", token.Error())
			return
		}
	}
}

func printLog(log PackLog) {
	fmt.Printf("[%s][%s][%s]: %s %s \r\n", log.LogTime, log.Level, log.From, log.Content, log.Error)
}
