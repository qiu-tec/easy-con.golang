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
	modulesChan chan string
}

func NewMqttMonitor(setting MonitorSetting) IMonitor {
	monitor := &mqttMonitor{
		MonitorSetting: setting,
		modulesChan:    make(chan string, 100),
	}
	adapterSetting := setting.Setting
	adapterSetting.OnLog = monitor.onAdapterLog
	adapterSetting.OnNotice = monitor.onAdapterNotice
	adapterSetting.OnRetainNotice = monitor.onAdapterRetainNotice
	adapter := newMqttAdapterInner(adapterSetting, monitor.onLinked)
	monitor.mqttAdapter = adapter
	go func() {
		for {
			module := <-monitor.modulesChan
			monitor.discoverModules(module)
		}
	}()
	return monitor
}

// 发现请求
func (monitor *mqttMonitor) onReqDetected(message mqtt.Message) {
	pack := &PackReq{}
	err := json.Unmarshal(message.Payload(), pack)
	if err != nil {
		monitor.Err("REQ Pack unmarshal error", err)
	}
	monitor.modulesChan <- pack.From
	monitor.modulesChan <- pack.To
	monitor.OnReqDetected(*pack)
}

// MonitorResp 主动响应
func (monitor *mqttMonitor) MonitorResp(req PackReq, respCode EResp, content any) {
	//resp := newRespPack(reqInner, respCode, content, err)
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
	monitor.modulesChan <- pack.From
	monitor.modulesChan <- pack.To
	monitor.OnRespDetected(*pack)
}

func (monitor *mqttMonitor) onLinked(_ mqtt.Client) {
	for _, module := range monitor.DetectiveModules {
		monitor.sub(module)

	}
}

func (monitor *mqttMonitor) onAdapterLog(log PackLog) {
	monitor.modulesChan <- log.From
	monitor.OnLog(log)
}

func (monitor *mqttMonitor) onAdapterNotice(notice PackNotice) {
	monitor.modulesChan <- notice.From
	monitor.OnNotice(notice)
}

func (monitor *mqttMonitor) onAdapterRetainNotice(notice PackNotice) {
	monitor.modulesChan <- notice.From
	monitor.OnRetainNotice(notice)
}

func (monitor *mqttMonitor) discoverModules(module string) {
	for _, v := range monitor.DetectiveModules {
		if v == module {
			return
		}

	}
	monitor.DetectiveModules = append(monitor.DetectiveModules, module)
	monitor.sub(module)
}

func (monitor *mqttMonitor) sub(module string) {
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
