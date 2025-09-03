/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/9/3 10:30
 */

package versionAndExit

import (
	"fmt"
	easyCon "github.com/qiu-tec/easy-con.golang"
	"testing"
	"time"
)

func TestVersionAndExit(t *testing.T) {
	setting := easyCon.NewSetting("Commander", "ws://127.0.0.1:5002/ws", nil, nil)
	cmd := easyCon.NewMqttAdapter(setting)

	setting.Module = "Worker"
	setting.OnGetVersion = func() []string {
		return []string{"Worker:V1.0.0"}
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
