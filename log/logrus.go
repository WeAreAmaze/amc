package log

import (
	"fmt"
	"github.com/go-stack/stack"
	"github.com/sirupsen/logrus"
	"os"
)

var (
	std = logrus.StandardLogger()
)

type logger struct {
	ctx []interface{}
}

func (l *logger) write(msg string, lvl Lvl, ctx []interface{}, skip int) {
	var field = make(map[string]interface{})
	field["prefix"] = fmt.Sprintf("%k", stack.Caller(skip))
	ctx = newContext(l.ctx, ctx)
	for i := 0; i < len(ctx); i += 2 {
		k, ok := ctx[i].(string)
		if !ok {

		}
		if s, ok := ctx[i+1].(TerminalStringer); ok {
			field[k] = s.TerminalString()
		} else {
			field[k] = ctx[i+1]
		}

	}
	switch lvl {
	case LvlCrit:
		terminal.WithFields(field).Panic(msg)
		std.WithFields(field).Panic(msg)
	case LvlError:
		terminal.WithFields(field).Error(msg)
		std.WithFields(field).Error(msg)
	case LvlWarn:
		terminal.WithFields(field).Warn(msg)
		std.WithFields(field).Warn(msg)
	case LvlInfo:
		terminal.WithFields(field).Info(msg)
		std.WithFields(field).Info(msg)
	case LvlDebug:
		terminal.WithFields(field).Debug(msg)
		std.WithFields(field).Debug(msg)
	case LvlTrace:
		terminal.WithFields(field).Trace(msg)
		std.WithFields(field).Trace(msg)
	}
}

func (l *logger) New(ctx ...interface{}) Logger {
	child := &logger{ctx: newContext(l.ctx, ctx)}
	return child
}

func newContext(prefix []interface{}, suffix []interface{}) []interface{} {
	normalizedSuffix := normalize(suffix)
	newCtx := make([]interface{}, len(prefix)+len(normalizedSuffix))
	n := copy(newCtx, prefix)
	copy(newCtx[n:], normalizedSuffix)
	return newCtx
}

func (l *logger) Trace(msg string, ctx ...interface{}) {
	l.write(msg, LvlTrace, ctx, skipLevel)
}

func (l *logger) Debug(msg string, ctx ...interface{}) {
	l.write(msg, LvlDebug, ctx, skipLevel)
}

func (l *logger) Info(msg string, ctx ...interface{}) {
	l.write(msg, LvlInfo, ctx, skipLevel)
}

func (l *logger) Warn(msg string, ctx ...interface{}) {
	l.write(msg, LvlWarn, ctx, skipLevel)
}

func (l *logger) Error(msg string, ctx ...interface{}) {
	l.write(msg, LvlError, ctx, skipLevel)
}

func (l *logger) Crit(msg string, ctx ...interface{}) {
	l.write(msg, LvlCrit, ctx, skipLevel)
	os.Exit(1)
}

func normalize(ctx []interface{}) []interface{} {
	// if the caller passed a Ctx object, then expand it
	if len(ctx) == 1 {
		if ctxMap, ok := ctx[0].(Ctx); ok {
			ctx = ctxMap.toArray()
		}
	}

	// ctx needs to be even because it's a series of key/value pairs
	// no one wants to check for errors on logging functions,
	// so instead of erroring on bad input, we'll just make sure
	// that things are the right length and users can fix bugs
	// when they see the output looks wrong
	if len(ctx)%2 != 0 {
		ctx = append(ctx, nil, "error", "Normalized odd number of arguments by adding nil")
	}

	return ctx
}

// Ctx is a map of key/value pairs to pass as context to a log function
// Use this only if you really need greater safety around the arguments you pass
// to the logging functions.
type Ctx map[string]interface{}

func (c Ctx) toArray() []interface{} {
	arr := make([]interface{}, len(c)*2)

	i := 0
	for k, v := range c {
		arr[i] = k
		arr[i+1] = v
		i += 2
	}

	return arr
}
