/**
 * @Author: Joey
 * @Description:
 * @Create Date: 2025/12/22 16:39
 */

package easyCon

//type monitor struct {
//	IAdapter
//}

func NewMqttMonitor(setting MqttSetting, callback AdapterCallBack) IAdapter {
	setting.IsRandomClientID = true
	setting.IsWaitLink = false
	setting.Module = "#"
	f := callback.OnLinked
	callback.OnLinked = func(adapter IAdapter) {

		if callback.OnNoticeRec != nil {
			adapter.SubscribeNotice("#", false)
		}
		if callback.OnRetainNoticeRec != nil {
			adapter.SubscribeNotice("#", true)
		}
		if f != nil {
			f(adapter)
		}
	}

	return newMqttAdapterInner(setting, callback)
}
func NewCGoMonitor(setting CoreSetting, callback AdapterCallBack, onRead func() []byte, onWrite func([]byte) error) IAdapter {
	setting.IsWaitLink = false
	setting.Module = "#"
	f := callback.OnLinked
	callback.OnLinked = func(adapter IAdapter) {

		if callback.OnNoticeRec != nil {
			adapter.SubscribeNotice("#", false)
		}
		if callback.OnRetainNoticeRec != nil {
			adapter.SubscribeNotice("#", true)
		}
		if f != nil {
			f(adapter)
		}
	}
	return NewCgoAdapter(setting, callback, onRead, onWrite)
}
