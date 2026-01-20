/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/9/3 19:02
 */

package main

import (
	"encoding/json"
	"fmt"
	easyCon "github.com/qiu-tec/easy-con.golang"
	"github.com/spf13/viper"
	"time"
)

func init() {
	cfgFile := "./proxy.yaml"
	if PathExists(cfgFile) == false {
		_ = WriteAllBytes(cfgFile, configData, false)
	}

	viper.SetConfigFile(cfgFile)
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	js, err := json.Marshal(viper.Get("ProxyConfig"))
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(js, &MyConfig)
	if err != nil {
		panic(err)
	}
	MyConfig.SettingA.TimeOut = MyConfig.SettingA.TimeOut * time.Millisecond
	MyConfig.SettingB.TimeOut = MyConfig.SettingB.TimeOut * time.Millisecond

}

func main() {
	var block = make(chan struct{})
	_ = easyCon.NewMqttProxy(MyConfig.SettingA, MyConfig.SettingB, MyConfig.ProxyNotice, MyConfig.ProxyRetainNotice, MyConfig.ProxyLog, MyConfig.ProxyModules)
	_ = <-block
}
