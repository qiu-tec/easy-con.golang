/**
 * @Author: Joey
 * @Description:适配器核心，实现了IAdapter，且提取访问器中与通信协议无关的逻辑代码作为通用代码进行封装
 * @Create Date: 2025/12/19 10:36
 */

package easyCon

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

// coreAdapter 适配器核心
type coreAdapter struct {
	setting            CoreSetting
	respDict           map[uint64]chan PackResp
	mu                 sync.RWMutex
	noticeChan         chan PackNotice
	retainNoticeChan   chan PackNotice
	reqChan            chan PackReq
	respChan           chan PackResp
	stopChan           chan interface{}
	logChan            chan PackLog
	startWaitChan      chan interface{}
	wg                 *sync.WaitGroup
	startOnce          sync.Once
	isLinked           bool
	noticeTopics       map[string]interface{}
	retainNoticeTopics map[string]interface{}
	engineCallback     EngineCallback
	adapterCallback    AdapterCallBack
}

// newCoreAdapter 创建 适配器核心
func newCoreAdapter(setting CoreSetting, engineCallback EngineCallback, adapterCallBack AdapterCallBack) *coreAdapter {
	var adapter *coreAdapter
	// 使用配置的缓冲区大小，默认为100
	bufferSize := setting.ChannelBufferSize
	if bufferSize <= 0 {
		bufferSize = 100
	}

	adapter = &coreAdapter{
		respDict:           make(map[uint64]chan PackResp),
		mu:                 sync.RWMutex{},
		reqChan:            make(chan PackReq, bufferSize),
		respChan:           make(chan PackResp, bufferSize),
		noticeChan:         make(chan PackNotice, bufferSize),
		retainNoticeChan:   make(chan PackNotice, bufferSize),
		stopChan:           make(chan interface{}),
		logChan:            make(chan PackLog, bufferSize),
		wg:                 &sync.WaitGroup{},
		noticeTopics:       make(map[string]interface{}),
		retainNoticeTopics: make(map[string]interface{}),
		engineCallback:     engineCallback,
		adapterCallback:    adapterCallBack,
		startWaitChan:      make(chan interface{}),
	}
	adapter.setting = setting
	adapter.link()

	return adapter
}
func (adapter *coreAdapter) waitLink() {
	if adapter.setting.IsWaitLink {
		_ = <-adapter.startWaitChan
	}
}

// Stop 停止
func (adapter *coreAdapter) Stop() {
	go func() {
		//time.Sleep(10)
		adapter.stopChan <- struct{}{}
	}()
	adapter.wg.Wait()
	if adapter.engineCallback.OnStop != nil {
		b, err := adapter.engineCallback.OnStop()
		if err != nil {
			adapter.sendLog(newLogPack(adapter.setting.Module, ELogLevelError, "OnStop error", err))
		}
		if b {
			if adapter.adapterCallback.OnStatusChanged != nil {
				adapter.adapterCallback.OnStatusChanged(EStatusStopped)
			}
			return
		}
	}
	if adapter.adapterCallback.OnStatusChanged != nil {
		adapter.adapterCallback.OnStatusChanged(EStatusStopped)
	}
}

// Reset 重启
func (adapter *coreAdapter) Reset() {
	adapter.Stop()
	adapter.link()
}

// Req 请求并等待响应
func (adapter *coreAdapter) Req(module, route string, params any) PackResp {
	if !adapter.isLinked {
		return PackResp{
			RespCode: ERespUnLinked,
		}
	}
	pack := newReqPack(adapter.setting.Module, module, route, params)
	for retry := adapter.setting.ReTry; retry > 0; retry-- {
		resp := adapter.reqInner(pack, 0)
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

// ReqWithTimeout 带超时的请求
func (adapter *coreAdapter) ReqWithTimeout(module, route string, params any, timeout int) PackResp {
	if !adapter.isLinked {
		return PackResp{
			RespCode: ERespUnLinked,
		}
	}
	pack := newReqPack(adapter.setting.Module, module, route, params)
	for retry := adapter.setting.ReTry; retry >= 0; retry-- {
		resp := adapter.reqInner(pack, timeout)
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

// SendRetainNotice 发送Retain消息
func (adapter *coreAdapter) SendRetainNotice(route string, content any) error {
	return adapter.sendNoticeInner(route, true, content)
}

// CleanRetainNotice 清除Retain消息
func (adapter *coreAdapter) CleanRetainNotice(route string) error {
	return adapter.sendNoticeInner(route, true, nil)
}

// SendNotice Send Notice
func (adapter *coreAdapter) SendNotice(route string, content any) error {
	return adapter.sendNoticeInner(route, false, content)
}
func (adapter *coreAdapter) SubscribeNotice(route string, isRetain bool) {
	if isRetain {
		topic := BuildRetainNoticeTopic(adapter.setting.PreFix, route)
		adapter.mu.Lock()
		adapter.retainNoticeTopics[topic] = topic
		adapter.mu.Unlock()
		adapter.engineCallback.OnSubscribe(topic, EPTypeNotice, func(pack IPack) {
			adapter.retainNoticeChan <- *pack.(*PackNotice)
		})
	} else {
		topic := BuildNoticeTopic(adapter.setting.PreFix, route)
		adapter.mu.Lock()
		adapter.noticeTopics[topic] = topic
		adapter.mu.Unlock()
		adapter.engineCallback.OnSubscribe(topic, EPTypeNotice, func(pack IPack) {
			adapter.noticeChan <- *pack.(*PackNotice)
		})
	}

}

func (adapter *coreAdapter) Debug(content string) {
	pack := newLogPack(adapter.setting.Module, ELogLevelDebug, content, nil)
	adapter.sendLog(pack)
}

func (adapter *coreAdapter) Warn(content string) {
	pack := newLogPack(adapter.setting.Module, ELogLevelWarning, content, nil)
	adapter.sendLog(pack)
}

func (adapter *coreAdapter) Err(content string, err error) {
	pack := newLogPack(adapter.setting.Module, ELogLevelError, content, err)
	adapter.sendLog(pack)
}

func (adapter *coreAdapter) Publish(topic string, isRetain bool, pack IPack) error {
	return adapter.engineCallback.OnPublish(topic, isRetain, pack)
}

// Inner func #########################################################################################
func (adapter *coreAdapter) reqInner(pack PackReq, timeout int) PackResp {
	tsp := adapter.setting.TimeOut
	if timeout > 0 {
		tsp = time.Duration(timeout) * time.Millisecond
	}

	pre := adapter.setting.PreFix
	to := pre + pack.To
	topic := BuildReqTopic(pre, to)

	// ✅ 先创建响应通道并注册，再发送请求，避免响应在注册前到达
	respChan := make(chan PackResp)
	adapter.mu.Lock()
	adapter.respDict[pack.Id] = respChan
	adapter.mu.Unlock()

	// 注册完成后再发送请求
	e := adapter.engineCallback.OnPublish(topic, false, &pack)
	if e != nil {
		// 发送失败，清理已注册的通道
		adapter.mu.Lock()
		delete(adapter.respDict, pack.Id)
		adapter.mu.Unlock()
		return PackResp{
			RespCode: ERespError,
			Error:    e.Error(),
		}
	}

	select {
	case resp := <-respChan:
		adapter.mu.Lock()
		delete(adapter.respDict, pack.Id)
		adapter.mu.Unlock()
		return resp
	case <-time.After(tsp):
		adapter.mu.Lock()
		delete(adapter.respDict, pack.Id)
		adapter.mu.Unlock()
		return PackResp{
			RespCode: ERespTimeout,
		}
	}
}

// sendNoticeInner 发消息核心代码
func (adapter *coreAdapter) sendNoticeInner(route string, isRetain bool, content any) error {
	pack := newNoticePack(adapter.setting.Module, route, content)
	topic := BuildNoticeTopic(adapter.setting.PreFix, route)
	if isRetain {
		topic = BuildRetainNoticeTopic(adapter.setting.PreFix, route)
	}
	return adapter.engineCallback.OnPublish(topic, isRetain, &pack)
}

// onRespRec handle response from respChan
func (adapter *coreAdapter) onRespRec(pack PackResp) {

	adapter.mu.RLock()
	c, b := adapter.respDict[pack.Id]
	if b {
		c <- pack
	}
	adapter.mu.RUnlock()
	if adapter.adapterCallback.OnRespRec != nil {
		adapter.adapterCallback.OnRespRec(pack)
	}
}

// onReqRec handle request from reqChan
func (adapter *coreAdapter) onReqRec(pack PackReq) {
	resp, content := adapter.adapterCallback.OnReqRec(pack)
	var respPack PackResp

	// content 现在是 []byte 类型，直接使用
	// 构造响应包时需要特殊处理，因为 newRespPack 期望 any 类型
	if content == nil {
		respPack = newRespPack(pack, resp, nil)
	} else {
		// 将 []byte 直接传入，newRespPack 会正确处理
		respPack = newRespPack(pack, resp, content)
	}

	if resp == ERespBypass { // 无需回复
		return
	}

	topic := BuildRespTopic(adapter.setting.PreFix, respPack.Target())
	e := adapter.engineCallback.OnPublish(topic, false, &respPack)
	if e != nil {
		adapter.Err("RESP send error", e)
		return
	}
}

func (adapter *coreAdapter) onReqInner(req PackReq) PackResp {
	if req.Route == "GetVersion" {
		var versions []string
		versions = append(versions, "easy-con:"+getVersion())
		if adapter.adapterCallback.OnGetVersion != nil {
			versions = append(versions, adapter.adapterCallback.OnGetVersion()...)
		}
		respPack := newRespPack(req, ERespSuccess, versions)
		return respPack
	}
	if req.Route == "Exit" {
		if adapter.adapterCallback.OnExiting != nil {
			adapter.adapterCallback.OnExiting()
		}
		respPack := newRespPack(req, ERespSuccess, nil)
		time.AfterFunc(time.Millisecond*100, func() {
			adapter.Stop()
		})
		return respPack
	}
	resp, content := adapter.adapterCallback.OnReqRec(req)
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
func (adapter *coreAdapter) onNoticeRec(pack PackNotice) {
	if adapter.adapterCallback.OnNoticeRec != nil && pack.From != adapter.setting.Module {
		adapter.adapterCallback.OnNoticeRec(pack)
	}
}
func (adapter *coreAdapter) onRetainNoticeRec(pack PackNotice) {
	if adapter.adapterCallback.OnRetainNoticeRec != nil && pack.From != adapter.setting.Module {
		adapter.adapterCallback.OnRetainNoticeRec(pack)
	}
}
func (adapter *coreAdapter) onLogRec(pack PackLog) {
	if adapter.adapterCallback.OnLogRec != nil && pack.From != adapter.setting.Module {
		adapter.adapterCallback.OnLogRec(pack)
	}
}
func (adapter *coreAdapter) subscribeAtLink() {
	if adapter.adapterCallback.OnReqRec != nil {
		adapter.subscribe("Req")
	}
	adapter.subscribe("Resp")
	//如果通知回调不为空，订阅通知主题
	if adapter.adapterCallback.OnNoticeRec != nil {
		for topic := range adapter.noticeTopics {
			adapter.subscribe(topic)
		}
	}
	if adapter.adapterCallback.OnRetainNoticeRec != nil {
		for topic := range adapter.retainNoticeTopics {
			adapter.subscribe(topic)
		}
	}
	//如果日志回调不为空，订阅日志主题
	if adapter.adapterCallback.OnLogRec != nil {
		adapter.subscribe("Log")
	}

}

// loop main loop
func (adapter *coreAdapter) loop() {
	adapter.engineCallback.OnLink()
	if adapter.adapterCallback.OnLinked != nil {
		adapter.adapterCallback.OnLinked(adapter)
	}
	adapter.wg.Add(1)
	defer adapter.wg.Done()
	if adapter.setting.IsSync {
		ch := make(chan struct{})
		go func() {
			for {
				select {
				case msg := <-adapter.respChan:
					go adapter.onRespRec(msg)
				case <-ch:
					return
				}
			}
		}()
		for {
			select {
			case msg := <-adapter.noticeChan:
				adapter.onNoticeRec(msg)
			case msg := <-adapter.retainNoticeChan:
				adapter.onRetainNoticeRec(msg)
			case msg := <-adapter.reqChan:
				adapter.onReqRec(msg)
			case msg := <-adapter.logChan:
				adapter.onLogRec(msg)
			case <-adapter.stopChan:
				ch <- struct{}{}
				return
			}
		}
	} else {
		for {
			select {
			case msg := <-adapter.noticeChan:
				go adapter.onNoticeRec(msg)
			case msg := <-adapter.retainNoticeChan:
				go adapter.onRetainNoticeRec(msg)
			case msg := <-adapter.reqChan:
				go adapter.onReqRec(msg)
			case msg := <-adapter.respChan:
				go adapter.onRespRec(msg)
			case msg := <-adapter.logChan:
				go adapter.onLogRec(msg)
			case <-adapter.stopChan:
				return
			}
		}
	}

}

func (adapter *coreAdapter) link() {
	go adapter.loop()

}

func (adapter *coreAdapter) sendLog(pack PackLog) {
	if adapter.setting.LogMode == ELogModeConsole || adapter.setting.LogMode == ELogModeAll {
		printLog(pack)
	}
	if adapter.setting.LogMode != ELogModeUpload && adapter.setting.LogMode != ELogModeAll {
		return
	}
	topic := adapter.setting.PreFix + LogTopic
	err := adapter.engineCallback.OnPublish(topic, false, &pack)
	if err != nil {
		pack.Error = err.Error() + " " + err.Error()
		pack.Level = ELogLevelError
		printLog(pack)
		return
	}
}

func printLog(log PackLog) {
	fmt.Printf("[%s][%s][%s]: %s %s \r\n", log.LogTime, log.Level, log.From, log.Content, log.Error)
}

func (adapter *coreAdapter) subscribe(kind string) {

	var topic string
	//var f func(IPack)
	switch kind {
	case "Req":
		topic = BuildReqTopic(adapter.setting.PreFix, adapter.setting.Module)
		adapter.engineCallback.OnSubscribe(topic, EPTypeReq, func(pack IPack) {
			adapter.reqChan <- *pack.(*PackReq)
		})
	case "Resp":
		topic = BuildRespTopic(adapter.setting.PreFix, adapter.setting.Module)
		adapter.engineCallback.OnSubscribe(topic, EPTypeResp, func(pack IPack) {
			adapter.respChan <- *pack.(*PackResp)
		})
	case "Notice":
		for t := range adapter.noticeTopics {
			adapter.engineCallback.OnSubscribe(t, EPTypeNotice, func(pack IPack) {
				adapter.noticeChan <- *pack.(*PackNotice)
			})
		}
	case "RetainNotice":
		for t := range adapter.retainNoticeTopics {
			adapter.engineCallback.OnSubscribe(t, EPTypeNotice, func(pack IPack) {
				adapter.retainNoticeChan <- *pack.(*PackNotice)
			})
		}

	case "Log":
		topic = BuildLogTopic(adapter.setting.PreFix)
		adapter.engineCallback.OnSubscribe(topic, EPTypeLog, func(pack IPack) {
			adapter.logChan <- *pack.(*PackLog)
		})
	}
}

// onConnected 当连接成功
func (adapter *coreAdapter) onConnected() {
	adapter.isLinked = true
	if adapter.setting.IsWaitLink {
		adapter.startOnce.Do(func() {
			close(adapter.startWaitChan)
		})
	}
	if adapter.adapterCallback.OnStatusChanged != nil {
		adapter.adapterCallback.OnStatusChanged(EStatusLinked)
	}
	err := adapter.SendNotice("Linked", "I am online")
	if err != nil {
		fmt.Printf("Send Notice error %s \r\n", err)
		return
	}
	adapter.subscribeAtLink()
}

func (adapter *coreAdapter) onReconnecting() {

	if adapter.adapterCallback.OnStatusChanged != nil {
		adapter.adapterCallback.OnStatusChanged(EStatusConnecting)
	}

}

func (adapter *coreAdapter) onConnectionLost(err error) {
	adapter.isLinked = false
	if adapter.adapterCallback.OnStatusChanged != nil {
		adapter.adapterCallback.OnStatusChanged(EStatusLinkLost)
	}
	fmt.Println("Connection lost because", err)
}
func (adapter *coreAdapter) GetEngineCallback() EngineCallback {
	return adapter.engineCallback
}
