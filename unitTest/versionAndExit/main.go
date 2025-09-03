/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/9/3 10:30
 */

package main

import (
	"fmt"
	easyCon "github.com/qiu-tec/easy-con.golang"
	"runtime/debug"
	"time"
)

func getVersion() string {
	// 尝试从构建信息中获取版本信息
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	// 如果无法从构建信息中获取，则回退到编译时注入的 Version 变量
	return "Unknown"
}
func main() {
	fmt.Println(getVersion())
	setting := easyCon.NewSetting("Commander", "ws://127.0.0.1:5002/ws", nil, nil)
	cmd := easyCon.NewMqttAdapter(setting)

	setting.Module = "Worker"
	setting.OnGetVersion = func() []string {
		return []string{"Worker:" + getVersion()}
	}
	setting.OnExiting = func() {
		fmt.Println("exiting")
		time.Sleep(time.Second)
		fmt.Println("ready to exit")
	}

	easyCon.NewMqttAdapter(setting)

	res := cmd.Req("Worker", "GetVersion", nil)
	fmt.Println(res)
	time.Sleep(time.Second)
	res = cmd.Req("Worker", "Exit", nil)
	fmt.Println(res)
	time.Sleep(time.Second)
	cmd.Stop()
}
