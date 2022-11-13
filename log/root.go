package log

import (
	"fmt"
	"github.com/amazechain/amc/conf"
	prefixed "github.com/amazechain/amc/log/logrus-prefixed-formatter"
	"github.com/sirupsen/logrus"
	"os"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

var (
	root     = &logger{[]interface{}{}}
	terminal = logrus.New()
)

type Lvl int

const skipLevel = 2

const (
	LvlCrit Lvl = iota
	LvlError
	LvlWarn
	LvlInfo
	LvlDebug
	LvlTrace
)

func Init(nodeConfig conf.NodeConfig, config conf.LoggerConfig) {
	formatter := new(prefixed.TextFormatter)
	formatter.TimestampFormat = "2006-01-02 15:04:05"
	formatter.FullTimestamp = true
	formatter.DisableColors = false
	logrus.SetFormatter(formatter)
	lvl, _ := logrus.ParseLevel(config.Level)
	logrus.SetLevel(lvl)

	jsonFormatter := new(logrus.JSONFormatter)
	jsonFormatter.TimestampFormat = "2006-01-02 15:04:05"
	terminal.SetFormatter(jsonFormatter)
	terminal.SetLevel(lvl)
	terminal.SetOutput(&lumberjack.Logger{
		Filename:   fmt.Sprintf("%s/log/%s", nodeConfig.DataDir, config.LogFile),
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   config.Compress,
	})
}

// New returns a new logger with the given context.
// New is a convenient alias for Root().New
func New(ctx ...interface{}) Logger {
	return root.New(ctx...)
}

// Root returns the root logger
func Root() Logger {
	return root
}

// Trace is a convenient alias for Root().Trace
func Trace(msg string, ctx ...interface{}) {
	root.write(msg, LvlTrace, ctx, skipLevel)
}

func Tracef(msg string, ctx ...interface{}) {
	root.write(fmt.Sprintf(msg, ctx), LvlTrace, []interface{}{}, skipLevel)
}

// Debug is a convenient alias for Root().Debug
func Debug(msg string, ctx ...interface{}) {
	root.write(msg, LvlDebug, ctx, skipLevel)
}

func Debugf(msg string, ctx ...interface{}) {
	root.write(fmt.Sprintf(msg, ctx), LvlDebug, []interface{}{}, skipLevel)
}

// Info is a convenient alias for Root().Info
func Info(msg string, ctx ...interface{}) {
	root.write(msg, LvlInfo, ctx, skipLevel)
}

// Infof is a convenient alias for Root().Info
func Infof(msg string, ctx ...interface{}) {
	root.write(fmt.Sprintf(msg, ctx), LvlInfo, []interface{}{}, skipLevel)
}

// Warn is a convenient alias for Root().Warn
func Warn(msg string, ctx ...interface{}) {
	root.write(msg, LvlWarn, ctx, skipLevel)
}

// Warnf is a convenient alias for Root().Warn
func Warnf(msg string, ctx ...interface{}) {
	root.write(fmt.Sprintf(msg, ctx), LvlWarn, []interface{}{}, skipLevel)
}

// Error is a convenient alias for Root().Error
func Error(msg string, ctx ...interface{}) {
	root.write(msg, LvlError, ctx, skipLevel)
}

// Errorf is a convenient alias for Root().Error
func Errorf(msg string, ctx ...interface{}) {
	root.write(fmt.Sprintf(msg, ctx), LvlError, []interface{}{}, skipLevel)
}

// Crit is a convenient alias for Root().Crit
func Crit(msg string, ctx ...interface{}) {
	root.write(msg, LvlCrit, ctx, skipLevel)
	os.Exit(1)
}

// Critf is a convenient alias for Root().Crit
func Critf(msg string, ctx ...interface{}) {
	root.write(fmt.Sprintf(msg, ctx), LvlCrit, []interface{}{}, skipLevel)
	os.Exit(1)
}

// A Logger writes key/value pairs to a Handler
type Logger interface {
	// New returns a new Logger that has this logger's context plus the given context
	New(ctx ...interface{}) Logger

	// Log a message at the given level with context key/value pairs
	Trace(msg string, ctx ...interface{})
	Debug(msg string, ctx ...interface{})
	Info(msg string, ctx ...interface{})
	Warn(msg string, ctx ...interface{})
	Error(msg string, ctx ...interface{})
	Crit(msg string, ctx ...interface{})
}

// TerminalStringer is an analogous interface to the stdlib stringer, allowing
// own types to have custom shortened serialization formats when printed to the
// screen.
type TerminalStringer interface {
	TerminalString() string
}
