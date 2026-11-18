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
	"runtime/debug"
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
	// 使用配置的缓冲区大小，默认为100
	bufferSize := setting.ChannelBufferSize
	if bufferSize <= 0 {
		bufferSize = 100
	}

	adapter = &mqttAdapter{
		respDict:         make(map[uint64]chan PackResp),
		mu:               sync.RWMutex{},
		reqChan:          make(chan mqtt.Message, bufferSize),
		respChan:         make(chan mqtt.Message, bufferSize),
		noticeChan:       make(chan mqtt.Message, bufferSize),
		retainNoticeChan: make(chan mqtt.Message, bufferSize),
		stopChan:         make(chan struct{}),
		logChan:          make(chan mqtt.Message, bufferSize),
		wg:               &sync.WaitGroup{},
	}

	adapter.setting = setting
	o := mqtt.NewClientOptions().
		SetClientID(setting.PreFix + setting.Module).
		AddBroker(setting.Addr).
		SetUsername(setting.UID).
		SetPassword(setting.PWD).
		SetAutoReconnect(true)
	o.OnConnect = func(client mqtt.Client) {
		adapter.isLinked = true
		if adapter.setting.StatusChanged != nil {
			adapter.setting.StatusChanged(EStatusLinked)
		}
		err := adapter.SendNotice("Linked", "I am online")
		if err != nil {
			fmt.Printf("Send Notice error %s \r\n", err)
			return
		}
		if adapter.setting.OnReq != nil {
			adapter.subscribe("Req")
		}

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
		if adapter.setting.StatusChanged != nil {
			adapter.setting.StatusChanged(EStatusLinkLost)
		}
	}
	o.OnReconnecting = func(client mqtt.Client, options *mqtt.ClientOptions) {
		if adapter.setting.StatusChanged != nil {
			adapter.setting.StatusChanged(EStatusConnecting)
		}
	}
	adapter.options = o
	adapter.link()
	return adapter
}

func (adapter *mqttAdapter) subscribe(kind string) {
	pre := adapter.setting.PreFix
	var topic string
	var channel chan mqtt.Message
	switch kind {
	case "Req":
		topic = buildReqTopic(pre + adapter.setting.Module)
		channel = adapter.reqChan
	case "Resp":
		topic = buildRespTopic(pre + adapter.setting.Module)
		channel = adapter.respChan
	case "Notice":
		topic = pre + NoticeTopic
		channel = adapter.noticeChan
	case "RetainNotice":
		topic = pre + RetainNoticeTopic
		channel = adapter.retainNoticeChan
	case "Log":
		topic = pre + LogTopic
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
	if adapter.setting.StatusChanged != nil {
		adapter.setting.StatusChanged(EStatusStopped)
	}
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
func (adapter *mqttAdapter) CleanRetainNotice() error {
	return adapter.sendNoticeInner("", true, nil)
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
		topic = adapter.setting.PreFix + RetainNoticeTopic
	}
	if route == "" && isRetain {
		js = nil
	}
	token := adapter.client.Publish(topic, 0, isRetain, js)
	if token.Wait() && token.Error() != nil {
		adapter.Err("Notice send error", token.Error())
		return token.Error()
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
	pre := adapter.setting.PreFix
	token := adapter.client.Publish(buildReqTopic(pre+pack.To), 0, false, raw)
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
		token := adapter.client.Publish(buildRespTopic(adapter.setting.PreFix+pack.From), 0, false, js)
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

func (adapter *mqttAdapter) onReqInner(req PackReq) PackResp {
	if req.Route == "GetVersion" {
		var versions []string
		versions = append(versions, "easy-con:"+getVersion())
		if adapter.setting.OnGetVersion != nil {
			versions = append(versions, adapter.setting.OnGetVersion()...)
		}
		respPack := newRespPack(req, ERespSuccess, versions)
		return respPack
	}
	if req.Route == "Exit" {
		if adapter.setting.OnExiting != nil {
			adapter.setting.OnExiting()
		}
		respPack := newRespPack(req, ERespSuccess, nil)
		time.AfterFunc(time.Millisecond*100, func() {
			adapter.Stop()
		})
		return respPack
	}
	resp, content := adapter.setting.OnReq(req)
	return newRespPack(req, resp, content)
}
func getVersion() string {
	// 尝试从构建信息中获取版本信息
	if info, ok := debug.ReadBuildInfo(); ok {
		// 首先检查主模块路径
		if info.Main.Path != "" && (info.Main.Path == "github.com/qiu-tec/easy-con.golang" ||
			info.Main.Path == "easy-con" || info.Main.Path == "easy-con.golang") {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				return info.Main.Version
			}
		}
		// 遍历所有的依赖项
		for _, dep := range info.Deps {
			// 匹配可能的包路径
			if dep.Path == "github.com/qiu-tec/easy-con.golang" ||
				dep.Path == "easy-con" ||
				dep.Path == "easy-con.golang" {
				return dep.Version
			}
		}
	}
	// 如果无法从构建信息中获取，则回退到默认版本
	return "0.0.0"
}
func (adapter *mqttAdapter) onNotice(message mqtt.Message) {
	if adapter.setting.OnNotice == nil {
		return
	}
	notice := &PackNotice{}
	err := json.Unmarshal(message.Payload(), notice)
	if err != nil {
		adapter.Err("Notice Unmarshal error", err)
	}
	if notice.From == adapter.setting.Module {
		return
	}
	adapter.setting.OnNotice(*notice)
}

func (adapter *mqttAdapter) onRetainNotice(message mqtt.Message) {
	if adapter.setting.OnRetainNotice == nil || message.Payload() == nil || len(message.Payload()) == 0 {
		return
	}

	notice := &PackNotice{}
	err := json.Unmarshal(message.Payload(), notice)
	if err != nil {
		adapter.Err("RetainNotice Unmarshal error", err)
	}
	if notice.From == adapter.setting.Module {
		return
	}
	adapter.setting.OnRetainNotice(*notice)
}

func (adapter *mqttAdapter) onLog(msg mqtt.Message) {
	if adapter.setting.OnLog == nil {
		return
	}
	var log PackLog
	err := json.Unmarshal(msg.Payload(), &log)
	if err != nil {
		fmt.Println("Log Unmarshal error", err)
	}
	//printLog(log)
	if log.From == adapter.setting.Module {
		return
	}
	adapter.setting.OnLog(log)
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
		// 使用配置的重试延迟，默认为1秒
		retryDelay := adapter.setting.ConnectRetryDelay
		if retryDelay <= 0 {
			retryDelay = time.Second
		}
		time.Sleep(retryDelay)
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

func printLog(log PackLog) {
	fmt.Printf("[%s][%s][%s]: %s %s \r\n", log.LogTime, log.Level, log.From, log.Content, log.Error)
}
