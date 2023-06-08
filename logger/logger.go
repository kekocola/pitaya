// Copyright (c) TFG Co. All Rights Reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package logger

import (
	"github.com/sirupsen/logrus"
	"github.com/topfreegames/pitaya/v2/logger/interfaces"
	logruswrapper "github.com/topfreegames/pitaya/v2/logger/logrus"
)

// Log is the default logger
var Log = initLogger()

func initLogger() interfaces.Logger {
	plog := logrus.New()
	plog.Formatter = new(logrus.TextFormatter)
	plog.Level = logrus.WarnLevel
	plog.SetReportCaller(true)

	// log := plog.WithFields(logrus.Fields{
	// 	"source": "pitaya",
	// })

	log := plog.WithFields(logrus.Fields{})

	return logruswrapper.NewWithFieldLogger(log)
}

// SetLogger rewrites the default logger
func SetLogger(l interfaces.Logger) {
	if l != nil {
		Log = l
	}
}

// // LogHook 日志钩子
// type LogHook struct {
// 	writerMap map[logrus.Level]*rotatelogs.RotateLogs
// }

// func NewLogHook() *LogHook {
// 	hook := &LogHook{
// 		writerMap: make(map[logrus.Level]*rotatelogs.RotateLogs),
// 	}

// 	errWriter, err := CreateLogWriter("err")
// 	if err != nil {
// 		logrus.Fatalf("create err log writer failed: %s", err.Error())
// 	}

// 	hook.writerMap[logrus.PanicLevel] = errWriter
// 	hook.writerMap[logrus.FatalLevel] = errWriter
// 	hook.writerMap[logrus.ErrorLevel] = errWriter

// 	infoWriter, err := CreateLogWriter("info")
// 	if err != nil {
// 		logrus.Fatalf("create info log writer failed: %s", err.Error())
// 	}

// 	hook.writerMap[logrus.WarnLevel] = infoWriter
// 	hook.writerMap[logrus.InfoLevel] = infoWriter
// 	hook.writerMap[logrus.DebugLevel] = infoWriter

// 	return hook
// }

// // hook Levels 接口
// func (hook *LogHook) Levels() []logrus.Level {
// 	return logrus.AllLevels
// }

// // hook Fire 接口
// func (hook *LogHook) Fire(entry *logrus.Entry) error {
// 	writer, ok := hook.writerMap[entry.Level]
// 	if !ok {
// 		return nil
// 	}

// 	msg, err := entry.String()
// 	if err != nil {
// 		return err
// 	}

// 	writer.Write([]byte(msg))
// 	return nil
// }
