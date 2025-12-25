/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/9/3 19:03
 */

package main

import (
	_ "embed"
	easyCon "github.com/qiu-tec/easy-con.golang"
)

//go:embed proxy.yaml
var configData []byte

type config struct {
	Mode              easyCon.EProxyMode
	ProxyNotice       bool
	ProxyRetainNotice bool
	ProxyLog          bool
	SettingA          easyCon.MqttProxySetting
	SettingB          easyCon.MqttProxySetting
	ProxyModules      []string
}

var MyConfig = config{
	Mode: easyCon.EProxyModeReverse,
	SettingA: easyCon.MqttProxySetting{
		Addr:    "ws://127.0.0.1:5002/ws",
		TimeOut: 1000,
		ReTry:   3,
		UID:     "",
		PWD:     "",
		PreFix:  "A.",
	},
	SettingB: easyCon.MqttProxySetting{
		Addr:    "ws://127.0.0.1:5002/ws",
		TimeOut: 1000,
		ReTry:   3,
		UID:     "",
		PWD:     "",
		PreFix:  "B.",
	},
}
