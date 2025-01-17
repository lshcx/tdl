package tclient

import (
	"github.com/lshcx/tdl/pkg/consts"
	"github.com/spf13/viper"
)

const (
	AppBuiltin = "builtin"
	AppDesktop = "desktop"
)

type App struct {
	AppID   int
	AppHash string
}

var Apps = map[string]App{
	// application created by iyear if flag is not set
	AppBuiltin: {AppID: viper.GetInt(consts.FlagAppID), AppHash: viper.GetString(consts.FlagAppHash)},
	// application created by tdesktop.
	// https://opentele.readthedocs.io/en/latest/documentation/authorization/api/#class-telegramdesktop
	AppDesktop: {AppID: 2040, AppHash: "b18441a1ff607e10a989891a5462e627"},
}
