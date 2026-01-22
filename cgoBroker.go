/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/12/25 11:20
 */

package easyCon

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type CgoBroker struct {
	clients map[string]func([]byte)
	topics  map[string]map[string]interface{} // topic -> module -> callback
	lock    sync.RWMutex
	onError func(err error)
	// onNoticeSubscribe 当模块订阅通知时的回调
	onNoticeSubscribe func(route, module string)
}

func NewCgoBroker() CgoBroker {
	return CgoBroker{
		clients: make(map[string]func([]byte)),
		topics:  make(map[string]map[string]interface{}),
	}
}
func (broker *CgoBroker) err(e error) {
	if broker.onError != nil {
		broker.onError(e)
	}
}

// SetNoticeSubscribeCallback 设置通知订阅回调
// 当有模块订阅通知时，会调用此回调，通知Proxy在MQTT Broker上订阅对应主题
func (broker *CgoBroker) SetNoticeSubscribeCallback(cb func(route, module string)) {
	broker.lock.Lock()
	defer broker.lock.Unlock()
	broker.onNoticeSubscribe = cb
}

func (broker *CgoBroker) generateTopic(pack IPack) string {
	switch p := pack.(type) {
	case *PackReq:
		return ReqTopic + p.To // "Request/" + To
	case *PackResp:
		// 对于响应，使用 To 字段（目标）而不是 From 字段（发送者）
		// 因为响应应该发送给目标（原始请求者）
		return RespTopic + p.To // "Response/" + To (目标模块)
	case *PackNotice:
		// 根据 Retain 字段决定使用哪个 topic
		if p.Retain {
			return RetainNoticeTopic + "/" + p.Route // "RetainNotice/" + Route
		}
		return NoticeTopic + "/" + p.Route // "Notice/" + Route
	case *PackLog:
		return LogTopic // "Log"
	}
	return ""
}

func (broker *CgoBroker) Publish(raw []byte) error {
	// 直接解析新协议格式
	pack, err := UnmarshalPack(raw)
	if err != nil {
		broker.err(err)
		return err
	}

	// 从Header生成topic
	topic := broker.generateTopic(pack)

	// 特殊处理：Broker请求
	if topic == "Request/Broker" {
		if reqPack, ok := pack.(*PackReq); ok {
			code, resp := broker.onReq(*reqPack)
			respPack := newRespPack(*reqPack, code, resp)
			js, _ := respPack.Raw()
			broker.onSend(BuildRespTopic("", reqPack.Target()), js)
			return nil
		}
	}

	broker.onSend(topic, raw)
	return nil
}

func (broker *CgoBroker) RegClient(id string, onRead func([]byte)) {
	now := time.Now().Format("15:04:05.000")
	broker.lock.Lock()
	defer broker.lock.Unlock()
	broker.clients[id] = onRead
	fmt.Printf("[%s][CgoBroker-RegClient] Registered module=%s onRead=%v\n", now, id, onRead != nil)
}

func (broker *CgoBroker) onSend(topic string, raw []byte) {
	now := time.Now().Format("15:04:05.000")
	fmt.Printf("[%s][CgoBroker-onSend] topic=%s\n", now, topic)
	var modulesToNotify []func([]byte)

	// 用于去重的 map，避免同一个模块被多次通知
	notifiedModules := make(map[string]bool)

	broker.lock.RLock()

	// 1. 首先检查精确匹配
	modules, ok := broker.topics[topic]
	if ok {
		for module, callback := range modules {
			if !notifiedModules[module] {
				// 如果 topics 中存储的是回调函数，直接使用
				if fn, ok := callback.(func([]byte)); ok {
					modulesToNotify = append(modulesToNotify, fn)
					notifiedModules[module] = true
				} else if callback == nil {
					// 如果是 nil 占位符，从 clients 中查找
					if f, b := broker.clients[module]; b {
						modulesToNotify = append(modulesToNotify, f)
						notifiedModules[module] = true
					}
				}
			}
		}
	}

	// 2. 检查通配符匹配 (MQTT 风格的 #)
	// 例如: Request/ModuleC 应该匹配 Request/#
	for subscribedTopic, modules := range broker.topics {
		if strings.HasSuffix(subscribedTopic, "/#") {
			// 提取通配符前缀
			prefix := strings.TrimSuffix(subscribedTopic, "/#")
			// 检查 topic 是否以该前缀开头
			if strings.HasPrefix(topic, prefix+"/") || topic == prefix {
				// 匹配成功，添加该主题下的所有订阅者
				for module, callback := range modules {
					if !notifiedModules[module] {
						if fn, ok := callback.(func([]byte)); ok {
							modulesToNotify = append(modulesToNotify, fn)
							notifiedModules[module] = true
						} else if callback == nil {
							if f, b := broker.clients[module]; b {
								modulesToNotify = append(modulesToNotify, f)
								notifiedModules[module] = true
							}
						}
					}
				}
			}
		}
	}

	broker.lock.RUnlock()
	fmt.Printf("[%s][CgoBroker-onSend] Found %d subscribers\n", now, len(modulesToNotify))

	// 直接传递新协议格式数据
	for _, f := range modulesToNotify {
		f(raw)
	}
	fmt.Printf("[%s][CgoBroker-onSend] Delivered to %d subscribers\n", now, len(modulesToNotify))
}
func (broker *CgoBroker) onReq(pack PackReq) (EResp, any) {
	switch pack.Route {
	case "Subscribe":
		var topic string
		err := json.Unmarshal(pack.Content, &topic)
		if err != nil {
			now := time.Now().Format("15:04:05.000")
			fmt.Printf("[%s][CgoBroker-onReq] Subscribe FAILED: Unmarshal error from %s\n", now, pack.From)
			return ERespBadReq, nil
		}
		now := time.Now().Format("15:04:05.000")
		fmt.Printf("[%s][CgoBroker-onReq] Subscribe request: Module=%s Topic=%s\n", now, pack.From, topic)

		broker.lock.Lock()
		defer broker.lock.Unlock()

		module := pack.From

		// 获取模块的回调函数（如果已注册）
		callback, registered := broker.clients[module]
		if !registered {
			fmt.Printf("[%s][CgoBroker-onReq] Subscribe: Module %s NOT registered with CgoBroker!\n", now, module)
		} else {
			fmt.Printf("[%s][CgoBroker-onReq] Subscribe: Module %s IS registered\n", now, module)
		}

		if broker.topics[topic] == nil {
			broker.topics[topic] = make(map[string]interface{})
		}
		m := broker.topics[topic]

		// 如果模块已注册，直接存储回调函数
		// 否则只存储模块名，等注册时再关联
		if registered {
			m[module] = callback
			fmt.Printf("[%s][CgoBroker-onReq] Subscribe: Registered %s for topic %s\n", now, module, topic)
		} else {
			m[module] = nil  // 占位符，表示已订阅但回调函数还未注册
			fmt.Printf("[%s][CgoBroker-onReq] Subscribe: Placeholder for %s on topic %s (waiting for registration)\n", now, module, topic)
		}

		// 检测是否为Notice订阅，如果是则通知Proxy
		// Topic格式: Notice/xxx 或 RetainNotice/xxx
		if broker.onNoticeSubscribe != nil && registered {
			if strings.HasPrefix(topic, NoticeTopic+"/") {
				route := strings.TrimPrefix(topic, NoticeTopic+"/")
				fmt.Printf("[%s][CgoBroker-onReq] Notice subscribe: Route=%s Module=%s\n", now, route, module)
				broker.onNoticeSubscribe(route, module)
			} else if strings.HasPrefix(topic, RetainNoticeTopic+"/") {
				route := strings.TrimPrefix(topic, RetainNoticeTopic+"/")
				fmt.Printf("[%s][CgoBroker-onReq] RetainNotice subscribe: Route=%s Module=%s\n", now, route, module)
				broker.onNoticeSubscribe(route, module)
			}
		}

		return ERespSuccess, nil
	default:
		return ERespRouteNotFind, nil
	}
}
