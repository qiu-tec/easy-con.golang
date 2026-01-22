/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2024/7/11 11:52
 */

package easyCon

import (
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"time"
)

type mqttAdapter struct {
	*coreAdapter
	client  mqtt.Client
	setting MqttSetting
	options *mqtt.ClientOptions
}

func NewMqttAdapter(setting MqttSetting, callback AdapterCallBack) IAdapter {
	return newMqttAdapterInner(setting, callback)
}

func newMqttAdapterInner(setting MqttSetting, callback AdapterCallBack) *mqttAdapter { // afterLink func(client mqtt.Client)
	adapter := &mqttAdapter{}
	ecb := EngineCallback{
		OnLink:       adapter.onLink,
		OnStop:       adapter.onStop,
		OnSubscribe:  adapter.onSubscribe,
		OnPublish:    adapter.onPublish,
		OnPublishRaw: adapter.PublishRaw,
	}

	adapter.setting = setting

	o := mqtt.NewClientOptions().
		AddBroker(setting.Addr).
		SetUsername(setting.UID).
		SetPassword(setting.PWD).
		SetAutoReconnect(true)

	// 配置MQTT特定的超时设置，优化WebSocket连接性能
	if setting.MqttKeepAlive > 0 {
		o.SetKeepAlive(setting.MqttKeepAlive)
	}
	if setting.MqttPingTimeout > 0 {
		o.SetPingTimeout(setting.MqttPingTimeout)
	}
	if setting.MqttWriteTimeout > 0 {
		o.SetWriteTimeout(setting.MqttWriteTimeout)
	}

	o.OnConnect = func(client mqtt.Client) {
		adapter.onConnected()
		//if afterLink != nil {
		//	afterLink(client)
		//}
	}
	o.OnConnectionLost = func(client mqtt.Client, err error) {
		adapter.onConnectionLost(err)
	}

	o.OnReconnecting = func(client mqtt.Client, options *mqtt.ClientOptions) {
		adapter.onReconnecting()

	}
	adapter.options = o
	adapter.coreAdapter = newCoreAdapter(setting.CoreSetting, ecb, callback)
	//等待连接成功。内部会根据配置阻塞
	adapter.coreAdapter.waitLink()
	return adapter
}

func (adapter *mqttAdapter) onStop() (isOk bool, err error) {
	defer func() {
		e := recover()
		if e != nil {
			isOk = false
			err = fmt.Errorf("mqtt client stop error %v", e)
		}
	}()
	adapter.client.Disconnect(100)
	isOk = true
	return
}

func (adapter *mqttAdapter) onPublish(topic string, isRetain bool, pack IPack) error {
	raw, err := pack.Raw()
	if err != nil {
		return err
	}

	token := adapter.client.Publish(topic, 0, isRetain, raw)
	// 异步发送：不等待确认，避免阻塞
	_ = token
	return nil
}

// PublishRaw publishes raw byte data (zero-copy)
func (adapter *mqttAdapter) PublishRaw(topic string, isRetain bool, data []byte) error {
	if adapter.client == nil {
		return fmt.Errorf("client is nil")
	}

	token := adapter.client.Publish(topic, 0, isRetain, data)
	// 异步发送：不等待确认，避免阻塞
	// if token.Wait() && token.Error() != nil {
	// 	return token.Error()
	// }
	_ = token
	return nil
}

// SubscribeInternalNotice Subscribe InternalNotice if route is "", route will be # and will subscribe all
func (adapter *mqttAdapter) onSubscribe(topic string, _ EPType, f func(pack IPack)) {
	adapter.client.Subscribe(topic, 0, func(_ mqtt.Client, message mqtt.Message) {
		pack, err := UnmarshalPack(message.Payload())
		if err != nil {
			adapter.Err("Deserialize error", err)
			return
		}
		f(pack)
	})
}

func (adapter *mqttAdapter) onLink() {
	suffix := ""
	//if adapter.setting.IsRandomClientID {
	//	suffix = "." + strconv.FormatInt(time.Now().UnixNano(), 10)
	//}
	adapter.options.SetClientID(adapter.setting.PreFix + adapter.setting.Module + suffix)
	adapter.client = mqtt.NewClient(adapter.options)
ReLink:
	token := adapter.client.Connect()
	if token.Wait() && token.Error() != nil {
		// 使用配置的重试延迟，默认为1秒
		retryDelay := adapter.setting.ConnectRetryDelay
		if retryDelay <= 0 {
			retryDelay = time.Second
		}
		time.Sleep(retryDelay)
		goto ReLink
	}
}
