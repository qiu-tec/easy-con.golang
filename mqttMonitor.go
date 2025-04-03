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
	"sync"
)

type mqttMonitor struct {
	client mqtt.Client
	MonitorSetting
	mu sync.RWMutex
	//stopChan chan struct{}
	options *mqtt.ClientOptions
	//wg       *sync.WaitGroup
	isLinked bool
}

func NewMqttMonitor(setting MonitorSetting) IMonitor {

	monitor := &mqttMonitor{
		MonitorSetting: setting,
		mu:             sync.RWMutex{},
		//stopChan:       make(chan struct{}),
		//wg: &sync.WaitGroup{},
	}

	o := mqtt.NewClientOptions().
		SetClientID(setting.Module).
		AddBroker(setting.Addr).
		SetUsername(setting.UID).
		SetPassword(setting.PWD).
		SetAutoReconnect(true)
	o.OnConnect = func(client mqtt.Client) {
		monitor.isLinked = true
		monitor.StatusChanged(EStatusLinked)
		topic := ""
		for _, module := range monitor.DetectiveModules {
			//订阅请求主题
			if monitor.OnReq != nil {
				topicReq := buildReqTopic(module)
				token := monitor.client.Subscribe(topicReq, 0, func(_ mqtt.Client, message mqtt.Message) {
					monitor.onReq(message)
				})
				if token.Wait() && token.Error() != nil {
					monitor.Err("Req Subscribe error", token.Error())
				}
			}
			if monitor.OnResp != nil {
				//订阅响应主题
				topicResp := buildRespTopic(module)
				token := monitor.client.Subscribe(topicResp, 0, func(_ mqtt.Client, message mqtt.Message) {
					monitor.onResp(message)
				})
				if token.Wait() && token.Error() != nil {
					monitor.Err("Resp Subscribe error", token.Error())
				}
			}
		}

		//如果通知回调不为空，订阅通知主题
		if monitor.OnNotice != nil {
			topic = NoticeTopic
			token := monitor.client.Subscribe(topic, 0, func(_ mqtt.Client, message mqtt.Message) {
				notice := &PackNotice{}
				err := json.Unmarshal(message.Payload(), notice)
				if err != nil {
					monitor.Err("Notice Unmarshal error", err)
				}
				monitor.OnNotice(*notice)
			})
			if token.Wait() && token.Error() != nil {
				monitor.Err("Notice Subscribe error", token.Error())
			}
			monitor.Debug(topic + " Subscribed")

		}
		if monitor.OnRetainNotice != nil {
			topic = RetainNoticeTopic
			token := monitor.client.Subscribe(topic, 0, func(_ mqtt.Client, message mqtt.Message) {
				notice := &PackNotice{}
				err := json.Unmarshal(message.Payload(), notice)
				if err != nil {
					monitor.Err("Notice Unmarshal error", err)
				}
				monitor.OnRetainNotice(*notice)
			})
			if token.Wait() && token.Error() != nil {
				monitor.Err("RetainNotice Subscribe error", token.Error())
			}
			monitor.Debug(topic + " Subscribed")
		}

		//如果日志回调不为空，订阅日志主题
		if monitor.OnLog != nil {
			topic = LogTopic
			token := monitor.client.Subscribe(topic, 0, func(_ mqtt.Client, message mqtt.Message) {
				log := &PackLog{}
				err := json.Unmarshal(message.Payload(), log)
				if err != nil {
					monitor.Err("Log Unmarshal error", err)
				}
				monitor.OnLog(*log)
			})
			if token.Wait() && token.Error() != nil {
				monitor.Err("Log Subscribe error", token.Error())
			}
			monitor.Debug(topic + " Subscribed")
		}

	}
	o.OnConnectionLost = func(client mqtt.Client, err error) {
		monitor.isLinked = false
		monitor.onStatusChanged(EStatusLinkLost)
	}
	o.OnReconnecting = func(client mqtt.Client, options *mqtt.ClientOptions) {
		monitor.onStatusChanged(EStatusConnecting)
	}

	monitor.options = o

	monitor.link()

	return monitor
}

func (monitor *mqttMonitor) Stop() {
	//go func() {
	//	//time.Sleep(10)
	//	monitor.stopChan <- struct{}{}
	//}()
	//monitor.wg.Wait()
	monitor.client.Disconnect(10)
	monitor.onStatusChanged(EStatusStopped)

}

func (monitor *mqttMonitor) Reset() {
	monitor.Stop()
	monitor.link()

}

func (monitor *mqttMonitor) link() {
	//go monitor.loop()
	monitor.client = mqtt.NewClient(monitor.options)
ReLink:
	token := monitor.client.Connect()
	if token.Wait() && token.Error() != nil {
		goto ReLink
	}
}

//func (monitor *mqttMonitor) loop() {
//	monitor.wg.Add(1)
//	defer monitor.wg.Done()
//	<-monitor.stopChan
//	return
//}

func (monitor *mqttMonitor) onStatusChanged(status EStatus) {
	if monitor.StatusChanged != nil {
		monitor.StatusChanged(status)
	}
}
func (monitor *mqttMonitor) Warn(content string) {
	pack := newLogPack(monitor.Module, ELogLevelWarning, content, nil)
	monitor.onLog(pack)
}

func (monitor *mqttMonitor) Err(content string, err error) {
	pack := newLogPack(monitor.Module, ELogLevelError, content, err)
	monitor.onLog(pack)
}
func (monitor *mqttMonitor) Debug(content string) {
	pack := newLogPack(monitor.Module, ELogLevelDebug, content, nil)
	monitor.onLog(pack)
}

// 请求chan
func (monitor *mqttMonitor) onReq(message mqtt.Message) {
	pack := &PackReq{}
	err := json.Unmarshal(message.Payload(), pack)
	if err != nil {
		monitor.Err("RESP unmarshal error", err)
	}
	monitor.Debug("Resp received raw is: " + string(message.Payload()))
	monitor.OnReq(*pack)
}

// 响应chan
func (monitor *mqttMonitor) onResp(message mqtt.Message) {

	pack := &PackResp{}
	err := json.Unmarshal(message.Payload(), pack)

	if err != nil {
		monitor.Err("RESP unmarshal error", err)
	}
	monitor.Debug("Resp received raw is: " + string(message.Payload()))
	monitor.OnResp(*pack)
}

func (monitor *mqttMonitor) sendLog(pack PackLog) {
	fmt.Printf("[%s][%s][%s]: %s %s \r\n", pack.LogTime, pack.Level, pack.From, pack.Content, pack.Error)

}

func (monitor *mqttMonitor) onLog(pack PackLog) {
	if monitor.OnLog == nil {
		monitor.OnLog(pack)
	}

}
