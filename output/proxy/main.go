/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/9/3 08:17
 */

package proxy

import (
	easyCon "github.com/qiu-tec/easy-con.golang"
	"os"
)

// 接收启动参数 确定加载的配置文件路径 如果为空则加载默认配置
func main() {
	if len(os.Args) > 1 {
		path := os.Args[1]
		// 加载配置

	}
	mySetting := loadFromFile()
	p := easyCon.NewMqttProxy(mySetting.SettingA, mySetting.SettingB, mySetting.Mode, mySetting.LogMode)

}
