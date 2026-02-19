package startup

import (
	"AstraScheduleServerGo/model"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"
)

func SetLog() {
	logrus.SetFormatter(&logrus.TextFormatter{
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			return "", fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line)
		},
	})
	logrus.SetReportCaller(true)
	if model.Configs.Log.Debug {
		logrus.SetLevel(logrus.TraceLevel)
	}
}
