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
	Mode     easyCon.EProxyMode
	SettingA easyCon.ProxySetting
	SettingB easyCon.ProxySetting
}

var MyConfig = config{
	Mode: easyCon.EProxyModeReverse,
	SettingA: easyCon.ProxySetting{
		Addr:    "ws://127.0.0.1:5002/ws",
		TimeOut: 1000,
		ReTry:   3,
		PreFix:  "A.",
	},
	SettingB: easyCon.ProxySetting{
		Addr:    "ws://127.0.0.1:5002/ws",
		TimeOut: 1000,
		ReTry:   3,
		PreFix:  "B.",
	},
}
