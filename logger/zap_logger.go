package logger

import (
	"io"
	"os"
	"path"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var ZapLog = initZapLogger()

func initZapLogger() *zap.SugaredLogger {
	encoderConfig := zap.NewProductionEncoderConfig()
	timeFormat := "2006-01-02 15:04:05.000"
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format(timeFormat))
	}
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	// 输出到日志文件
	cores := []zapcore.Core{}
	for level := zap.InfoLevel; level <= zapcore.FatalLevel; level++ {
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(CreateLogWriter(level.String())), getLevelPriority(level)))
	}

	// 调试模式输出log到控制台
	// if global.IsDebug {
	// 	encoderDev := zapcore.NewConsoleEncoder(zap.NewProductionEncoderConfig())
	// 	cores = append(cores, zapcore.NewCore(encoderDev, zapcore.AddSync(os.Stderr), zap.DebugLevel))
	// }
	core := zapcore.NewTee(cores...)

	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.WarnLevel), zap.AddCallerSkip(1))
	return logger.Sugar()
}

func getLevelPriority(level zapcore.Level) zap.LevelEnablerFunc {
	switch level {
	case zapcore.DebugLevel:
		return func(level zapcore.Level) bool { // 调试级别
			return level == zap.DebugLevel
		}
	case zapcore.InfoLevel:
		return func(level zapcore.Level) bool { // 日志级别
			return level == zap.InfoLevel
		}
	case zapcore.WarnLevel:
		return func(level zapcore.Level) bool { // 警告级别
			return level == zap.WarnLevel
		}
	case zapcore.ErrorLevel:
		return func(level zapcore.Level) bool { // 错误级别
			return level == zap.ErrorLevel
		}
	case zapcore.DPanicLevel:
		return func(level zapcore.Level) bool { // dpanic级别
			return level == zap.DPanicLevel
		}
	case zapcore.PanicLevel:
		return func(level zapcore.Level) bool { // panic级别
			return level == zap.PanicLevel
		}
	case zapcore.FatalLevel:
		return func(level zapcore.Level) bool { // 终止级别
			return level == zap.FatalLevel
		}
	default:
		return func(level zapcore.Level) bool { // 默认log无效
			return false
		}
	}
}

// CreateWriter 创建按照日期格式的日志文件io.Writer
func CreateLogWriter(filePrefix string) io.Writer {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	writer, err := rotatelogs.New(
		path.Join(cwd, "log", filePrefix+"-%Y-%m-%d.log"), // 日期格式的日志文件
		rotatelogs.WithClock(rotatelogs.Local),
		rotatelogs.WithMaxAge(time.Hour*24*3), //过期时间
	)

	if err != nil {
		panic(err)
	}

	return writer
}
