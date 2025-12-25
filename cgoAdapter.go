/**
 * @Author: Joey
 * @Description:CGO Plugin方式的访问器
 * @Create Date: 2025/12/19 10:34
 */

package easyCon

import (
	"encoding/binary"
	"fmt"
	"time"
)

type topicBack struct {
	EType EPType
	Func  func(IPack)
}
type cgoAdapter struct {
	*coreAdapter
	topics map[string]topicBack
	cgoClient
}

func NewCgoAdapter(setting CoreSetting, callback AdapterCallBack, onRead func() []byte, onWrite func([]byte) error) IAdapter {

	adapter := &cgoAdapter{
		cgoClient: cgoClient{
			onRead:  onRead,
			onWrite: onWrite,
		},
	}
	ecb := EngineCallback{
		OnLink:      func() { return },
		OnStop:      func() (bool, error) { return true, nil },
		OnSubscribe: adapter.onSubscribe,
		OnPublish:   adapter.onPublish,
	}

	adapter.coreAdapter = newCoreAdapter(setting, ecb, callback)
	adapter.readLoop()
	return adapter
}

type IClient struct {
	OnSubscribe func(topic string, pType EPType, f func(IPack))
	OnPublish   func(topic string, retain bool, pack IPack) error
}
type cgoClient struct {
	onRead  func() []byte
	onWrite func([]byte) error
}

func (adapter *cgoAdapter) readLoop() {
	for {
		rawPack := adapter.onRead()
		topic, raw, err := unMarshalCgoPack(rawPack)
		if err != nil {
			adapter.Err("Deserialize error", err)
			continue
		}
		t, b := adapter.topics[topic]
		var pack IPack
		if b {
			switch t.EType {
			case EPTypeReq:
				pack, err = unmarshalPack(EPTypeReq, raw)
				if err != nil {
					adapter.Err("Deserialize Req error", err)
				}
			case EPTypeResp:
				pack, err = unmarshalPack(EPTypeResp, raw)
				if err != nil {
					adapter.Err("Deserialize Resp error", err)
				}
			case EPTypeNotice:
				pack, err = unmarshalPack(EPTypeNotice, raw)
				if err != nil {
					adapter.Err("Deserialize Notice error", err)
				}
			case EPTypeLog:
				pack, err = unmarshalPack(EPTypeLog, raw)
				if err != nil {
					adapter.Err("Deserialize Log error", err)
				}
			default:
				adapter.Err("unknown topic", fmt.Errorf("unknown topic %s", topic))
			}
		}
		t.Func(pack)
	}
}
func marshalCgoPack(topic string, raw []byte) []byte {
	topicBytes := []byte(topic)
	length := uint32(len(topicBytes))
	buf := make([]byte, 4+len(topicBytes)+len(raw))
	binary.BigEndian.PutUint32(buf, length)
	copy(buf[4:], topicBytes)
	copy(buf[4+len(topicBytes):], raw)
	return buf
}
func unMarshalCgoPack(pack []byte) (string, []byte, error) {
	length := binary.BigEndian.Uint32(pack[:4])
	if length == 0 {
		return "", nil, fmt.Errorf("empty pack")
	}
	if len(pack) < 4+int(length) {
		return "", nil, fmt.Errorf("pack too short")
	}
	return string(pack[4 : 4+length]), pack[4+length:], nil

}
func (adapter *cgoAdapter) onSubscribe(topic string, pType EPType, f func(IPack)) {
	adapter.topics[topic] = topicBack{EType: pType, Func: f}
	pack := newReqPack(adapter.setting.Module, "Broker", "Subscribe", topic)
	js, _ := pack.Raw()
	cgoRaw := marshalCgoPack(BuildReqTopic(adapter.setting.PreFix, "Broker"), js)
	err := adapter.onWrite(cgoRaw)
	if err != nil {
		fmt.Printf("[%s]:CGO Subscribe Failed because %s\r\n", time.Now().Format("15:04:05.000"), err.Error())
	}
}
func (adapter *cgoAdapter) onPublish(topic string, _ bool, pack IPack) error {
	js, _ := pack.Raw()
	cgoRaw := marshalCgoPack(topic, js)
	return adapter.onWrite(cgoRaw)
}
