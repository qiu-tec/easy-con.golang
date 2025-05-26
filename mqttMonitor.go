/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/4/3 11:27
 */

package easyCon

import (
	"encoding/json"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type mqttMonitor struct {
	MonitorSetting
	options  *mqtt.ClientOptions
	isLinked bool
	*mqttAdapter
}

func NewMqttMonitor(setting MonitorSetting) IMonitor {
	monitor := &mqttMonitor{
		MonitorSetting: setting,
	}
	adapter := newMqttAdapterWithAfterLink(setting.Setting, monitor.onLinked)
	monitor.mqttAdapter = adapter
	return monitor
}

// 发现请求
func (monitor *mqttMonitor) onReqDetected(message mqtt.Message) {
	pack := &PackReq{}
	err := json.Unmarshal(message.Payload(), pack)
	if err != nil {
		monitor.Err("REQ Pack unmarshal error", err)
	}
	monitor.OnReqDetected(*pack)
}

// MonitorResp 主动响应
func (monitor *mqttMonitor) MonitorResp(req PackReq, respCode EResp, content any) {
	//resp := newRespPack(req, respCode, content, err)
	respPack := newRespPack(req, respCode, content)
	js, err := json.Marshal(respPack)
	if err != nil {
		monitor.Err("RESP marshal error", err)
		return
	}
	token := monitor.client.Publish(buildRespTopic(respPack.From), 0, false, js)
	if token.Wait() && token.Error() != nil {
		monitor.Err("RESP send error", token.Error())
		return
	}
}

// 发现响应
func (monitor *mqttMonitor) onRespDetected(message mqtt.Message) {

	pack := &PackResp{}
	err := json.Unmarshal(message.Payload(), pack)

	if err != nil {
		monitor.Err("RESP unmarshal error", err)
	}
	monitor.OnRespDetected(*pack)
}

func (monitor *mqttMonitor) onLinked(_ mqtt.Client) {
	for _, module := range monitor.DetectiveModules {
		//订阅请求主题
		if monitor.OnReqDetected != nil {
			topicReq := buildReqTopic(module)
			token := monitor.client.Subscribe(topicReq, 0, func(_ mqtt.Client, message mqtt.Message) {
				monitor.onReqDetected(message)
			})
			if token.Wait() && token.Error() != nil {
				monitor.Err("Req Subscribe error", token.Error())
			}
		}
		if monitor.OnRespDetected != nil {
			//订阅响应主题
			topicResp := buildRespTopic(module)
			token := monitor.client.Subscribe(topicResp, 0, func(_ mqtt.Client, message mqtt.Message) {
				monitor.onRespDetected(message)
			})
			if token.Wait() && token.Error() != nil {
				monitor.Err("Resp Subscribe error", token.Error())
			}
		}
	}
}
