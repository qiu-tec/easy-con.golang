/**
 * @Author: Joey
 * @Description:CGO Plugin方式的访问器
 * @Create Date: 2025/12/19 10:34
 */

package easyCon

import (
	"fmt"
	"time"
)

type topicBack struct {
	EType EPType
	Func  func(IPack)
}
type cgoAdapter struct {
	*coreAdapter
	topics   map[string]topicBack
	onWrite  func([]byte) error
	readChan chan []byte
}

func (adapter *cgoAdapter) onRead(raw []byte) {
	adapter.readChan <- raw
}
func NewCgoAdapter(setting CoreSetting, callback AdapterCallBack, onWrite func([]byte) error) (IAdapter, func([]byte)) {

	adapter := &cgoAdapter{
		onWrite:  onWrite,
		topics:   make(map[string]topicBack),
		readChan: make(chan []byte, setting.ChannelBufferSize),
	}
	ecb := EngineCallback{
		OnLink:       func() { return },
		OnStop:       func() (bool, error) { return true, nil },
		OnSubscribe:  adapter.onSubscribe,
		OnPublish:    adapter.onPublish,
		OnPublishRaw: adapter.PublishRaw,
	}

	adapter.coreAdapter = newCoreAdapter(setting, ecb, callback)
	go adapter.readLoop()
	adapter.onConnected()
	return adapter, adapter.onRead
}

func (adapter *cgoAdapter) generateTopic(pack IPack) string {
	prefix := adapter.setting.PreFix
	switch p := pack.(type) {
	case *PackReq:
		return BuildReqTopic(prefix, p.To)
	case *PackResp:
		return BuildRespTopic(prefix, p.From)
	case *PackNotice:
		// 根据 Retain 字段决定使用哪个 topic
		if p.Retain {
			return BuildRetainNoticeTopic(prefix, p.Route)
		}
		return BuildNoticeTopic(prefix, p.Route)
	case *PackLog:
		return BuildLogTopic(prefix)
	}
	return ""
}

type IClient struct {
	OnSubscribe func(topic string, pType EPType, f func(IPack))
	OnPublish   func(topic string, retain bool, pack IPack) error
}

//type cgoClient struct {
//	onRead  func() []byte
//	onWrite func([]byte) error
//}

func (adapter *cgoAdapter) readLoop() {
	for {
		select {
		case <-adapter.stopChan:
			return
		case rawPack := <-adapter.readChan:
			// 直接解析新协议格式
			pack, err := UnmarshalPack(rawPack)
			if err != nil {
				adapter.Err("Deserialize error", err)
				continue
			}

			// 从Header生成topic
			topic := adapter.generateTopic(pack)

			// 匹配处理函数
			adapter.mu.Lock()
			t, b := adapter.topics[topic]
			adapter.mu.Unlock()

			if b {
				t.Func(pack)
			} else {
				// 尝试monitor topic
				tt := getMonitorTopic(topic)
				adapter.mu.Lock()
				t, b = adapter.topics[tt]
				adapter.mu.Unlock()
				if b {
					t.Func(pack)
				}
			}
		}
	}
}

func (adapter *cgoAdapter) onSubscribe(topic string, pType EPType, f func(IPack)) {
	adapter.mu.Lock()
	adapter.topics[topic] = topicBack{EType: pType, Func: f}
	adapter.mu.Unlock()

	pack := newReqPack(adapter.setting.Module, "Broker", "Subscribe", topic)
	js, _ := pack.Raw()
	// To字段为"Broker"，接收方会生成"Request/Broker"topic
	err := adapter.onWrite(js)
	if err != nil {
		fmt.Printf("[%s]:CGO Subscribe Failed because %s\r\n", time.Now().Format("15:04:05.000"), err.Error())
	}
}
func (adapter *cgoAdapter) onPublish(topic string, _ bool, pack IPack) error {
	js, _ := pack.Raw()
	return adapter.onWrite(js)
}

// PublishRaw publishes raw byte data (zero-copy)
func (adapter *cgoAdapter) PublishRaw(topic string, isRetain bool, data []byte) error {
	return adapter.onWrite(data)
}
