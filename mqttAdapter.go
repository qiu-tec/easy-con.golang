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
	"sync/atomic"
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
	isLinked         atomic.Bool
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
		SetAutoReconnect(true).
		SetWriteTimeout(setting.TimeOut) // D5 修复：未设置时 paho Publish 内部走 30s time.After，定时器堆积
	o.OnConnect = func(client mqtt.Client) {
		adapter.isLinked.Store(true)
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
		adapter.isLinked.Store(false)
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
	if !adapter.isLinked.Load() {
		return PackResp{
			RespCode: ERespUnLinked,
		}
	}
	// 每次尝试生成新的 pack.Id（getReqId 全局自增，Id 本就是「每次请求唯一」的语义）：
	// 1. 对端可据此区分不同尝试，为幂等去重提供依据
	// 2. respDict 注册不会互相覆盖，避免通道泄漏与响应张冠李戴
	var lastPack PackReq
	for retry := adapter.setting.ReTry; retry >= 0; retry-- {
		pack := newReqPack(adapter.setting.Module, module, route, params)
		lastPack = pack
		resp := adapter.reqInner(pack)
		switch resp.RespCode {
		case ERespTimeout:
			continue
		default:
			return resp
		}
	}
	p := PackResp{
		PackReq:  lastPack,
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
	if !adapter.isLinked.Load() {
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

	now := time.Now()
	if !token.WaitTimeout(adapter.setting.TimeOut) {
		err = fmt.Errorf("notice timeout cfg timeout=%d, use timeout=%d", adapter.setting.TimeOut, time.Now().Sub(now))
		adapter.Err(fmt.Sprintf("Notice %d send timeout\r\n", pack.Id), err)
		return err
	}
	if token.Error() != nil {
		adapter.Err(fmt.Sprintf("Notice %d send error\r\n", pack.Id), token.Error())
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
	// 先注册响应通道，再发布请求，消除「响应早于注册导致响应被静默丢弃」的竞态窗口
	// 通道带 1 缓冲：onResp 投递永不阻塞，避免持 RLock 阻塞导致适配器僵死
	respChan := make(chan PackResp, 1)
	adapter.mu.Lock()
	adapter.respDict[pack.Id] = respChan
	adapter.mu.Unlock()
	// 无论成功/超时/错误，统一从字典移除，避免分支遗漏导致字典泄漏
	defer func() {
		adapter.mu.Lock()
		delete(adapter.respDict, pack.Id)
		adapter.mu.Unlock()
	}()

	pre := adapter.setting.PreFix
	token := adapter.client.Publish(buildReqTopic(pre+pack.To), 0, false, raw)
	now := time.Now()
	if !token.WaitTimeout(adapter.setting.TimeOut) {
		adapter.Err(fmt.Sprintf("REQ %d send timeout\r\n", pack.Id),
			fmt.Errorf("cfg timeout=%d, use timeout=%d", adapter.setting.TimeOut, time.Now().Sub(now)))
		return PackResp{
			RespCode: ERespTimeout,
		}
	}
	if token.Error() != nil {
		adapter.Err(fmt.Sprintf("REQ %d send error\r\n", pack.Id), token.Error())
		return PackResp{
			RespCode: ERespUnLinked,
		}
	}
	// D5 修复：time.After 无法提前释放，高吞吐下 30s/5s 存活定时器堆积数百 MB；
	// 改为 NewTimer + Stop，收到响应立即释放
	timer := time.NewTimer(adapter.setting.TimeOut)
	defer timer.Stop()
	select {
	case resp := <-respChan:
		return resp
	case <-timer.C:
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
		// 非阻塞投递：缓冲已满时丢弃，避免持 RLock 阻塞导致字典写入全部卡死
		select {
		case c <- *pack:
		default:
		}
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
		now := time.Now()
		if !token.WaitTimeout(adapter.setting.TimeOut) {
			adapter.Err(fmt.Sprintf("RESP %d send timeout\r\n", pack.Id),
				fmt.Errorf("cfg timeout=%d, use timeout=%d", adapter.setting.TimeOut, time.Now().Sub(now)))
			return
		}
		if token.Error() != nil {
			adapter.Err(fmt.Sprintf("RESP %d send error\r\n", pack.Id), token.Error())
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
	// wg.Add 由 link()/NewMqttAdapterWithClient 在启动本 goroutine 之前完成，
	// 保证 Stop() 的 wg.Wait() 不会早于 Add 执行（Go 规范要求 Add 先于 Wait）
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
	adapter.wg.Add(1) // 在启动 goroutine 前 Add，保证 Stop() 的 Wait 语义正确
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

// NewMqttAdapterWithClient 使用外部提供的 mqtt.Client 构造适配器（供单元测试注入模拟客户端）
// 与 NewMqttAdapter 的区别：不建立真实连接、不发送 Linked 通知，
// 但会完成主题订阅并启动消息循环，使收发回路可被测试驱动。
// 注意：调用方需自行保证 client 的生命周期
func NewMqttAdapterWithClient(setting Setting, client mqtt.Client) IAdapter {
	adapter := &mqttAdapter{
		client:           client,
		setting:          setting,
		respDict:         make(map[uint64]chan PackResp),
		mu:               sync.RWMutex{},
		reqChan:          make(chan mqtt.Message, 100),
		respChan:         make(chan mqtt.Message, 100),
		noticeChan:       make(chan mqtt.Message, 100),
		retainNoticeChan: make(chan mqtt.Message, 100),
		logChan:          make(chan mqtt.Message, 100),
		stopChan:         make(chan struct{}),
		wg:               &sync.WaitGroup{},
	}
	adapter.isLinked.Store(true)
	adapter.wg.Add(1)
	go adapter.loop()

	// 订阅逻辑与 OnConnect 回调保持一致
	if adapter.setting.OnReq != nil {
		adapter.subscribe("Req")
	}
	adapter.subscribe("Resp")
	if adapter.setting.OnNotice != nil {
		adapter.subscribe("Notice")
	}
	if adapter.setting.OnRetainNotice != nil {
		adapter.subscribe("RetainNotice")
	}
	if adapter.setting.OnLog != nil {
		adapter.subscribe("Log")
	}
	return adapter
}

func (adapter *mqttAdapter) sendLog(pack PackLog) {
	if adapter.setting.LogMode == ELogModeConsole || adapter.setting.LogMode == ELogModeAll {
		printLog(pack)
	}
	if adapter.setting.LogMode == ELogModeUpload || adapter.setting.LogMode == ELogModeAll {

		if adapter.isLinked.Load() == false { //未连接 就不再继续尝试发包了
			return
		}
		js, err := json.Marshal(pack)
		if err != nil {
			fmt.Printf("[%s][%s][Error]: %s\r\n", time.Now().Format("2006-01-02 15:04:05.000"), adapter.setting.Module, err.Error())
			return
		}
		adapter.client.Publish(adapter.setting.PreFix+LogTopic, 0, false, js)
		//token := adapter.client.Publish(adapter.setting.PreFix+LogTopic, 0, false, js)
		//if !token.WaitTimeout(adapter.setting.LogTimeOut) {
		//	// 注意：WaitTimeout 超时时 paho 不会给 token 设置 error，Error() 可能为 nil，
		//	// 不能直接调用 token.Error().Error()，否则会空指针 panic
		//	fmt.Printf("[%s][%s][Timeout]: log publish timeout cfg timeout=%d\r\n",
		//		time.Now().Format("2006-01-02 15:04:05.000"), adapter.setting.Module, adapter.setting.LogTimeOut)
		//	return
		//}
		//if token.Error() != nil {
		//	fmt.Printf("[%s][%s][Error]: %s\r\n", time.Now().Format("2006-01-02 15:04:05.000"), adapter.setting.Module, token.Error().Error())
		//	return
		//}
	}
}

func printLog(log PackLog) {
	fmt.Printf("[%s][%s][%s]: %s %s \r\n", log.LogTime, log.Level, log.From, log.Content, log.Error)
}
