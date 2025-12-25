/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/12/25 11:20
 */

package easyCon

import (
	"fmt"
	"strings"
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
	if topic == "Broker" { //需要broker处理的请求
		pack, e := unmarshalPack(EPTypeReq, raw)
		if e != nil {
			broker.err(e)
		}
		code, resp := broker.onReq(*pack.(*PackReq))
		respPack := newRespPack(*pack.(*PackReq), code, resp)
		js, _ := respPack.Raw()
		respCgoRaw := marshalCgoPack(BuildRespTopic("", pack.Target()), js)
		broker.onSend(topic, respCgoRaw)
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
	broker.lock.RLock()
	defer broker.lock.RUnlock()
	modules, ok := broker.topics[topic]

	if !ok {
		broker.err(fmt.Errorf("unknown topic %s", topic))
	}
	for module := range modules {
		f, b := broker.clients[module]
		if !b {
			broker.err(fmt.Errorf("unknown module %s", module))
		}
		f(cgoRaw)
	}
	monitorTopic := broker.getMonitorTopic(topic)
	modules, ok = broker.topics[monitorTopic]
	if !ok {
		broker.err(fmt.Errorf("unknown topic %s", topic))
	}
	for module := range modules {
		f, b := broker.clients[module]
		if !b {
			broker.err(fmt.Errorf("unknown module %s", module))
		}
		f(cgoRaw)
	}
}
func (broker *CgoBroker) onReq(pack PackReq) (EResp, any) {
	switch pack.Route {
	case "Subscribe":
		topic := pack.Content.(string)
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

func (broker *CgoBroker) getMonitorTopic(topic string) string {
	sections := strings.Split(topic, "/")
	sections = sections[:len(sections)-1]
	return strings.Join(sections, "/") + "#"
}
