/**
 * @Author: Joey
 * @Description:CGO Plugin方式的访问器
 * @Create Date: 2025/12/19 10:34
 */

package easyCon

import (
	"fmt"
	"strings"
	"time"
)

type topicBack struct {
	EType EPType
	Func  func(IPack)
}
type cgoAdapter struct {
	*coreAdapter
	topics       map[string]topicBack
	onWrite      func([]byte) error // External write (to MQTT broker)
	localBroker  func([]byte) error // Local write (to CgoBroker)
	readChan     chan []byte
}

func (adapter *cgoAdapter) onRead(raw []byte) {
	adapter.readChan <- raw
}
func NewCgoAdapter(setting CoreSetting, callback AdapterCallBack, onWrite func([]byte) error) (IAdapter, func([]byte)) {
	// 默认情况下，localBroker 与 onWrite 相同
	// 这样订阅请求也会通过 onWrite 发送
	return NewCgoAdapterWithBroker(setting, callback, onWrite, onWrite)
}

func NewCgoAdapterWithBroker(setting CoreSetting, callback AdapterCallBack, onWrite func([]byte) error, localBroker func([]byte) error) (IAdapter, func([]byte)) {

	adapter := &cgoAdapter{
		onWrite:     onWrite,
		localBroker: localBroker,
		topics:      make(map[string]topicBack),
		readChan:    make(chan []byte, setting.ChannelBufferSize),
	}
	ecb := EngineCallback{
		OnLink:       func() { adapter.onConnected() },
		OnStop:       func() (bool, error) { return true, nil },
		OnSubscribe:  adapter.onSubscribe,
		OnPublish:    adapter.onPublish,
		OnPublishRaw: adapter.PublishRaw,
	}

	adapter.coreAdapter = newCoreAdapter(setting, ecb, callback)
	go adapter.readLoop()
	return adapter, adapter.onRead
}

func (adapter *cgoAdapter) generateTopic(pack IPack) string {
	prefix := adapter.setting.PreFix
	switch p := pack.(type) {
	case *PackReq:
		return BuildReqTopic(prefix, p.To)
	case *PackResp:
		// 对于响应，使用 To 字段（目标）而不是 From 字段（发送者）
		return BuildRespTopic(prefix, p.To)
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
			now := time.Now().Format("15:04:05.000")
			// 直接解析新协议格式
			pack, err := UnmarshalPack(rawPack)
			if err != nil {
				adapter.Err("Deserialize error", err)
				continue
			}

			// 从Header生成topic
			topic := adapter.generateTopic(pack)
			fmt.Printf("[%s][CgoAdapter-readLoop] Module=%s Received pack: topic=%s type=%s\n", now, adapter.setting.Module, topic, pack.GetType())

			// 匹配处理函数
			adapter.mu.Lock()
			t, b := adapter.topics[topic]
			adapter.mu.Unlock()

			if b {
				fmt.Printf("[%s][CgoAdapter-readLoop] Exact match found for topic=%s, calling callback\n", now, topic)
				t.Func(pack)
			} else {
				// 尝试通配符匹配：从最长前缀开始，逐步缩短
				matched := false
				parts := strings.Split(topic, "/")
				for i := len(parts); i >= 1; i-- {
					wildcardTopic := strings.Join(parts[:i], "/") + "/#"
					adapter.mu.Lock()
					t, b = adapter.topics[wildcardTopic]
					adapter.mu.Unlock()
					if b {
						fmt.Printf("[%s][CgoAdapter-readLoop] Wildcard match found: %s -> %s, calling callback\n", now, topic, wildcardTopic)
						t.Func(pack)
						matched = true
						break
					}
				}
				if !matched {
					fmt.Printf("[%s][CgoAdapter-readLoop] No match found for topic=%s\n", now, topic)
				}
			}
		}
	}
}

func (adapter *cgoAdapter) onSubscribe(topic string, pType EPType, f func(IPack)) {
	adapter.mu.Lock()
	adapter.topics[topic] = topicBack{EType: pType, Func: f}
	adapter.mu.Unlock()

	now := time.Now().Format("15:04:05.000")
	fmt.Printf("[%s][cgoAdapter-onSubscribe] Module=%s Topic=%s localBroker=%v\n", now, adapter.setting.Module, topic, adapter.localBroker != nil)

	// 如果有 localBroker（CgoBroker），发送订阅请求到本地 Broker
	if adapter.localBroker != nil {
		pack := newReqPack(adapter.setting.Module, "Broker", "Subscribe", topic)
		js, _ := pack.Raw()
		// To字段为"Broker"，接收方会生成"Request/Broker"topic
		err := adapter.localBroker(js)
		if err != nil {
			fmt.Printf("[%s]:CGO Subscribe Failed because %s\r\n", now, err.Error())
		} else {
			fmt.Printf("[%s][cgoAdapter-onSubscribe] Sent subscribe request to CgoBroker: topic=%s\n", now, topic)
		}
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
