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

type mqttAdapter struct {
	client           mqtt.Client
	setting          Setting
	respDict         map[uint64]chan PackResp
	mu               sync.RWMutex
	noticeChan       chan mqtt.Message
	retainNoticeChan chan mqtt.Message
	reqChan          chan mqtt.Message
	respChan         chan mqtt.Message
	stopChan         chan struct{}
	logChan          chan mqtt.Message
	options          *mqtt.ClientOptions
	wg               *sync.WaitGroup
	isLinked         bool
}

func NewMqttAdapter(setting Setting) IAdapter {
	return newMqttAdapterInner(setting, nil)
}

func newMqttAdapterInner(setting Setting, afterLink func(client mqtt.Client)) *mqttAdapter {
	var adapter *mqttAdapter
	adapter = &mqttAdapter{
		respDict:         make(map[uint64]chan PackResp),
		mu:               sync.RWMutex{},
		reqChan:          make(chan mqtt.Message, 10),
		respChan:         make(chan mqtt.Message, 10),
		noticeChan:       make(chan mqtt.Message, 100),
		retainNoticeChan: make(chan mqtt.Message, 100),
		stopChan:         make(chan struct{}),
		logChan:          make(chan mqtt.Message, 100),
		wg:               &sync.WaitGroup{},
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
		err := adapter.SendNotice("Linked", "I am online")
		if err != nil {
			fmt.Printf("Send Notice error %s \r\n", err)
			return
		}
		adapter.subscribe("Req")
		adapter.subscribe("Resp")
		//如果通知回调不为空，订阅通知主题
		if adapter.setting.OnNotice != nil {
			adapter.subscribe("Notice")
		}
		if adapter.setting.OnRetainNotice != nil {
			adapter.subscribe("RetainNotice")
		}
		//如果日志回调不为空，订阅日志主题
		if adapter.setting.OnLog != nil {
			adapter.subscribe("Log")
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

func (adapter *mqttAdapter) subscribe(kind string) {
	var topic string
	var channel chan mqtt.Message
	switch kind {
	case "Req":
		topic = buildReqTopic(adapter.setting.Module)
		channel = adapter.reqChan
	case "Resp":
		topic = buildRespTopic(adapter.setting.Module)
		channel = adapter.respChan
	case "Notice":
		topic = adapter.setting.PreFix + NoticeTopic
		channel = adapter.noticeChan
	case "RetainNotice":
		topic = adapter.setting.PreFix + RetainNoticeTopic
		channel = adapter.retainNoticeChan
	case "Log":
		topic = adapter.setting.PreFix + LogTopic
		channel = adapter.logChan
	}

	token := adapter.client.Subscribe(topic, 0, func(_ mqtt.Client, message mqtt.Message) {
		channel <- message
	})
	if token.Wait() && token.Error() != nil {
		adapter.Err(kind+" Subscribe error", token.Error())
	}
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

// Req Send Request and wait response
func (adapter *mqttAdapter) Req(module, route string, params any) PackResp {
	if !adapter.isLinked {
		return PackResp{
			RespCode: ERespUnLinked,
		}
	}
	pack := newReqPack(adapter.setting.Module, module, route, params)
	for retry := adapter.setting.ReTry; retry >= 0; retry-- {
		resp := adapter.reqInner(pack)
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

// SendRetainNotice Send Retain Notice
func (adapter *mqttAdapter) SendRetainNotice(route string, content any) error {
	return adapter.sendNoticeInner(route, true, content)
}

// SendNotice Send Notice
func (adapter *mqttAdapter) SendNotice(route string, content any) error {
	return adapter.sendNoticeInner(route, false, content)
}

// sendNoticeInner Send Notice inner
func (adapter *mqttAdapter) sendNoticeInner(route string, isRetain bool, content any) error {
	if !adapter.isLinked {
		return errors.New("adapter is not linked")
	}
	var pack PackNotice
	if isRetain {
		pack = newRetainNoticePack(adapter.setting.Module, route, content)
	} else {
		pack = newNoticePack(adapter.setting.Module, route, content)
	}

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
	adapter.sendLog(pack)
}

func (adapter *mqttAdapter) Warn(content string) {
	pack := newLogPack(adapter.setting.Module, ELogLevelWarning, content, nil)
	adapter.sendLog(pack)
}

func (adapter *mqttAdapter) Err(content string, err error) {
	pack := newLogPack(adapter.setting.Module, ELogLevelError, content, err)
	adapter.sendLog(pack)
}

func (adapter *mqttAdapter) reqInner(pack PackReq) PackResp {
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

// onResp handle response from respChan
func (adapter *mqttAdapter) onResp(message mqtt.Message) {

	pack := &PackResp{}
	err := json.Unmarshal(message.Payload(), pack)
	if err != nil {
		adapter.Err("RESP unmarshal error", err)
	}
	if pack.RespCode != ERespSuccess && pack.Content != nil {

		e, b := pack.Content.(error)
		if b {
			pack.Error = e.Error()
		} else {
			s, b := pack.Content.(string)
			if b {
				pack.Error = s
			} else {
				pack.Error = fmt.Sprintf("%v", pack.Content)
			}
		}
		pack.Content = nil
	}
	adapter.mu.RLock()
	c, b := adapter.respDict[pack.Id]
	if b {
		c <- *pack
	}
	adapter.mu.RUnlock()
}

// onReq handle request from reqChan
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
	}()

	err := json.Unmarshal(message.Payload(), pack)
	if err != nil {
		respPack = newRespPack(*pack, ERespBadReq, err)
		return
	}
	resp, content := adapter.setting.OnReq(*pack)
	respPack = newRespPack(*pack, resp, content)

}

func (adapter *mqttAdapter) onNotice(message mqtt.Message) {
	notice := &PackNotice{}
	err := json.Unmarshal(message.Payload(), notice)
	if err != nil {
		adapter.Err("Notice Unmarshal error", err)
	}
	adapter.setting.OnNotice(*notice)
}

func (adapter *mqttAdapter) onRetainNotice(message mqtt.Message) {
	notice := &PackNotice{}
	err := json.Unmarshal(message.Payload(), notice)
	if err != nil {
		adapter.Err("Notice Unmarshal error", err)
	}
	adapter.setting.OnRetainNotice(*notice)
}

// loop main loop
func (adapter *mqttAdapter) loop() {
	adapter.wg.Add(1)
	defer adapter.wg.Done()
	if adapter.setting.EProtocol == EProtocolMQTTSync {
		ch := make(chan struct{})
		go func() {
			for {
				select {
				case msg := <-adapter.respChan:
					go adapter.onResp(msg)
				case msg := <-adapter.logChan:
					go adapter.onLog(msg)
				case <-ch:
					return

				}
			}
		}()
		for {
			select {
			case msg := <-adapter.noticeChan:
				adapter.onNotice(msg)
			case msg := <-adapter.retainNoticeChan:
				adapter.onRetainNotice(msg)
			case msg := <-adapter.reqChan:
				adapter.onReq(msg)
			case msg := <-adapter.logChan:
				adapter.onLog(msg)
			case <-adapter.stopChan:
				ch <- struct{}{}
				return
			}
		}
	} else {
		for {
			select {
			case msg := <-adapter.noticeChan:
				go adapter.onNotice(msg)
			case msg := <-adapter.retainNoticeChan:
				go adapter.onRetainNotice(msg)
			case msg := <-adapter.reqChan:
				go adapter.onReq(msg)
			case msg := <-adapter.respChan:
				go adapter.onResp(msg)
			case msg := <-adapter.logChan:
				go adapter.onLog(msg)
			case <-adapter.stopChan:
				return
			}
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
			fmt.Printf("[%s][%s][Error]: %s\r\n", time.Now().Format("2006-01-02 15:04:05.000"), adapter.setting.Module, token.Error().Error())
			return
		}
	}
}

func (adapter *mqttAdapter) onLog(msg mqtt.Message) {
	var log PackLog
	err := json.Unmarshal(msg.Payload(), &log)
	if err != nil {
		fmt.Println("Log Unmarshal error", err)
	}
	printLog(log)
}

func printLog(log PackLog) {
	fmt.Printf("[%s][%s][%s]: %s %s \r\n", log.LogTime, log.Level, log.From, log.Content, log.Error)
}
