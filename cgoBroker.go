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
	topics  map[string]map[string]interface{}
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
func (broker *CgoBroker) Publish(cgoRaw []byte) error {
	topic, raw, err := unMarshalCgoPack(cgoRaw)
	if err != nil {
		broker.err(err)
	}
	if topic == "Request/Broker" { //需要broker处理的请求
		pack, e := UnmarshalPack(raw)
		if e != nil {
			broker.err(e)
			return e
		}
		code, resp := broker.onReq(*pack.(*PackReq))
		respPack := newRespPack(*pack.(*PackReq), code, resp)
		js, _ := respPack.Raw()
		respCgoRaw := marshalCgoPack(BuildRespTopic("", pack.Target()), js)
		broker.onSend(topic, respCgoRaw)
		return nil
	}
	broker.onSend(topic, cgoRaw)
	return nil
}

func (broker *CgoBroker) RegClient(id string, onRead func([]byte)) {
	broker.lock.Lock()
	defer broker.lock.Unlock()
	broker.clients[id] = onRead
}

func (broker *CgoBroker) onSend(topic string, cgoRaw []byte) {

	var modulesToNotify []func([]byte)

	broker.lock.RLock()
	modules, ok := broker.topics[topic]
	if ok {
		for module := range modules {
			f, b := broker.clients[module]
			if b {
				modulesToNotify = append(modulesToNotify, f)
			}

		}
	}
	broker.lock.RUnlock()

	monitorTopic := getMonitorTopic(topic)
	broker.lock.RLock()

	modules, ok = broker.topics[monitorTopic]
	if ok {
		for module := range modules {
			f, b := broker.clients[module]
			if b {
				modulesToNotify = append(modulesToNotify, f)
			}
		}
	}
	broker.lock.RUnlock()
	// 在锁外执行消息发送
	for _, f := range modulesToNotify {
		f(cgoRaw)
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
		if broker.topics[topic] == nil {
			broker.topics[topic] = make(map[string]interface{})
		}
		m := broker.topics[topic]
		m[pack.From] = pack.From
		return ERespSuccess, nil
	default:
		return ERespRouteNotFind, nil
	}
}
