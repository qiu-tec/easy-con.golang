/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/12/25 11:20
 */

package easyCon

import (
	"encoding/json"
	"sync"
)

type CgoBroker struct {
	clients map[string]func([]byte)
	topics  map[string]map[string]interface{} // topic -> module -> callback
	lock    sync.RWMutex
	onError func(err error)
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

func (broker *CgoBroker) generateTopic(pack IPack) string {
	switch p := pack.(type) {
	case *PackReq:
		return ReqTopic + p.To // "Request/" + To
	case *PackResp:
		return RespTopic + p.From // "Response/" + From
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
	broker.lock.Lock()
	defer broker.lock.Unlock()
	broker.clients[id] = onRead
}

func (broker *CgoBroker) onSend(topic string, raw []byte) {
	var modulesToNotify []func([]byte)

	broker.lock.RLock()
	modules, ok := broker.topics[topic]
	if ok {
		for module, callback := range modules {
			// 如果 topics 中存储的是回调函数，直接使用
			if fn, ok := callback.(func([]byte)); ok {
				modulesToNotify = append(modulesToNotify, fn)
			} else if callback == nil {
				// 如果是 nil 占位符，从 clients 中查找
				if f, b := broker.clients[module]; b {
					modulesToNotify = append(modulesToNotify, f)
				}
			}
		}
	}
	broker.lock.RUnlock()

	monitorTopic := getMonitorTopic(topic)
	broker.lock.RLock()

	modules, ok = broker.topics[monitorTopic]
	if ok {
		for module, callback := range modules {
			// 如果 topics 中存储的是回调函数，直接使用
			if fn, ok := callback.(func([]byte)); ok {
				modulesToNotify = append(modulesToNotify, fn)
			} else if callback == nil {
				// 如果是 nil 占位符，从 clients 中查找
				if f, b := broker.clients[module]; b {
					modulesToNotify = append(modulesToNotify, f)
				}
			}
		}
	}
	broker.lock.RUnlock()

	// 直接传递新协议格式数据
	for _, f := range modulesToNotify {
		f(raw)
	}
}
func (broker *CgoBroker) onReq(pack PackReq) (EResp, any) {
	switch pack.Route {
	case "Subscribe":
		var topic string
		err := json.Unmarshal(pack.Content, &topic)
		if err != nil {
			return ERespBadReq, nil
		}
		broker.lock.Lock()
		defer broker.lock.Unlock()

		module := pack.From

		// 获取模块的回调函数（如果已注册）
		callback, registered := broker.clients[module]

		if broker.topics[topic] == nil {
			broker.topics[topic] = make(map[string]interface{})
		}
		m := broker.topics[topic]

		// 如果模块已注册，直接存储回调函数
		// 否则只存储模块名，等注册时再关联
		if registered {
			m[module] = callback
		} else {
			m[module] = nil  // 占位符，表示已订阅但回调函数还未注册
		}

		return ERespSuccess, nil
	default:
		return ERespRouteNotFind, nil
	}
}
