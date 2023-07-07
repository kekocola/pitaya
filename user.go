package pitaya

import "github.com/topfreegames/pitaya/v2/interfaces"

// check user online status
func (app *App) IsUserOnline(uid uint64) bool {
	m, ok := app.modulesMap["bindingsStorage"]
	if !ok {
		return false
	}
	return m.(interfaces.BindingStorage).IsUserOnline(uid)
}
