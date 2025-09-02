/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/4/3 11:27
 */

package easyCon

import (
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type mqttMonitor struct {
	MonitorSetting
	options  *mqtt.ClientOptions
	isLinked bool
	*mqttAdapter
	modulesChan chan string
	discovered  []string
	onDiscover  func(module string)
}

func (monitor *mqttMonitor) Discover(module string) {
	monitor.modulesChan <- module
}

func NewMqttMonitor(setting MonitorSetting) IMonitor {
	monitor := &mqttMonitor{
		MonitorSetting: setting,
		modulesChan:    make(chan string, 100),
		discovered:     make([]string, 0),
		onDiscover:     setting.OnDiscover,
	}
	adapterSetting := setting.Setting
	adapterSetting.OnLog = monitor.onAdapterLog
	adapterSetting.OnNotice = monitor.onAdapterNotice
	adapterSetting.OnRetainNotice = monitor.onAdapterRetainNotice
	monitor.discovered = append(monitor.discovered, monitor.DetectiveModules...)

	adapter := newMqttAdapterInner(adapterSetting, monitor.onLinked)
	monitor.mqttAdapter = adapter
	go func() {
		for {
			module := <-monitor.modulesChan
			monitor.discover(module)
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
	respPack := newRespPack(req, respCode, content)
	js, err := json.Marshal(respPack)
	if err != nil {
		monitor.Err("RESP marshal error", err)
		return
	}
	token := monitor.client.Publish(buildRespTopic(monitor.PreFix+respPack.From), 0, false, js)
	if token.Wait() && token.Error() != nil {
		monitor.Err("RESP send error", token.Error())
		return
	}
}
func (monitor *mqttMonitor) MonitorNotice(notice PackNotice) {
	js, err := json.Marshal(notice)
	if err != nil {
		monitor.Err("Notice marshal error", err)
		return
	}
	token := monitor.client.Publish(monitor.PreFix+NoticeTopic, 0, false, js)
	if token.Wait() && token.Error() != nil {
		monitor.Err("Notice send error", token.Error())
		return
	}
}
func (monitor *mqttMonitor) MonitorRetainNotice(notice PackNotice) {
	js, err := json.Marshal(notice)
	if err != nil {
		monitor.Err("Notice marshal error", err)
		return
	}
	token := monitor.client.Publish(monitor.PreFix+RetainNoticeTopic, 0, true, js)
	if token.Wait() && token.Error() != nil {
		monitor.Err("Retain Notice send error", token.Error())
		return
	}
}
func (monitor *mqttMonitor) MonitorLog(log PackLog) {
	js, err := json.Marshal(log)
	if err != nil {
		monitor.Err("Notice marshal error", err)
		return
	}
	token := monitor.client.Publish(monitor.PreFix+LogTopic, 0, false, js)
	if token.Wait() && token.Error() != nil {
		monitor.Err("Log send error", token.Error())
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
	for _, module := range monitor.discovered {
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

func (monitor *mqttMonitor) discover(module string) {
	if module == monitor.Module {
		return
	}
	// 如果设置了需要发现的模块 那么就不需要发现了
	if monitor.DetectiveModules != nil && len(monitor.DetectiveModules) > 0 {
		return
	}
	for _, v := range monitor.discovered {
		if v == module {
			return
		}
	}
	monitor.discovered = append(monitor.discovered, module)
	monitor.sub(module)
	if monitor.onDiscover != nil {
		monitor.onDiscover(module)
	}
}

func (monitor *mqttMonitor) sub(module string) {

	pre := monitor.PreFix
	//订阅请求主题
	if monitor.OnReqDetected != nil {
		topicReq := buildReqTopic(pre + module)
		token := monitor.client.Subscribe(topicReq, 0, func(_ mqtt.Client, message mqtt.Message) {
			monitor.onReqDetected(message)
		})
		if token.Wait() && token.Error() != nil {
			monitor.Err("Req Subscribe error", token.Error())
		}
	}
	if monitor.OnRespDetected != nil {
		//订阅响应主题
		topicResp := buildRespTopic(pre + module)
		token := monitor.client.Subscribe(topicResp, 0, func(_ mqtt.Client, message mqtt.Message) {
			monitor.onRespDetected(message)
		})
		if token.Wait() && token.Error() != nil {
			monitor.Err("Resp Subscribe error", token.Error())
		}
	}

	fmt.Println("Module", module, "discovered by", monitor.Module)
}
