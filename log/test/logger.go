// Copyright 2022 The AmazeChain Authors
// This file is part of the AmazeChain library.
//
// The AmazeChain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The AmazeChain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the AmazeChain library. If not, see <http://www.gnu.org/licenses/>.

package test

import (
	"fmt"
	"github.com/amazechain/amc/conf"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	lumberjackV2 "gopkg.in/natefinch/lumberjack.v2"
	"os"
	"strings"
	"sync"
)

var (
	Logger      *zap.Logger
	sugarLogger *zap.SugaredLogger
	once        sync.Once
	level       zapcore.Level
)

func init() {
	encode := getEncoder()
	core := zapcore.NewCore(encode, zapcore.AddSync(os.Stdout), zapcore.DebugLevel)
	Logger = zap.New(core, zap.AddCaller())
	sugarLogger = Logger.Sugar()
	sugarLogger.Debugf("abcd %d, %s, %d", 1, "2", 3)
}

func Init(config *conf.LoggerConfig) error {
	if config == nil {
		config = &conf.LoggerConfig{
			LogFile:    "./amc.log",
			Level:      "debug",
			MaxSize:    10,
			MaxBackups: 10,
			MaxAge:     30,
			Compress:   false,
		}
	}

	if err := level.Set(config.Level); err != nil {
		return err
	}

	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= level && lvl < zapcore.ErrorLevel
	})

	errorLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})

	once.Do(func() {
		writeSyncer := getLogWriter(config)
		encode := getEncoder()
		//core := zapcore.NewCore(encode, writeSyncer, level)
		core := zapcore.NewTee(
			zapcore.NewCore(encode, writeSyncer, infoLevel),
			zapcore.NewCore(encode, zapcore.AddSync(os.Stdout), level),
			zapcore.NewCore(encode, getErrorWriter(config), errorLevel),
		)

		Logger = zap.New(core, zap.AddCaller())
		sugarLogger = Logger.Sugar()
	})

	return nil
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	//encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	//encoderConfig.CallerKey = "caller"
	//encoderConfig.NameKey = "logger"
	//encoderConfig.EncodeName = zapcore.FullNameEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func getLumberjack(path string, maxSize, maxBackups, maxAge int, compress bool) *lumberjackV2.Logger {
	return &lumberjackV2.Logger{
		Filename:   path,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   compress,
	}
}

func getLogWriter(config *conf.LoggerConfig) zapcore.WriteSyncer {
	return zapcore.AddSync(getLumberjack(config.LogFile, config.MaxSize, config.MaxBackups, config.MaxAge, config.Compress))
}

func getErrorWriter(config *conf.LoggerConfig) zapcore.WriteSyncer {
	var name string
	i := strings.LastIndex(config.LogFile, ".")
	if i <= 0 {
		name = fmt.Sprintf("%s_error.log", config.LogFile)
	} else {
		name = fmt.Sprintf("%s_error.log", config.LogFile[:i])
	}
	return zapcore.AddSync(getLumberjack(name, config.MaxSize, config.MaxBackups, config.MaxAge, config.Compress))
}

func Debug(args ...interface{}) {
	sugarLogger.Debug(args)
}

// Info uses fmt.Sprint to construct and log a message.
func Info(args ...interface{}) {
	sugarLogger.Info(args)
}

// Warn uses fmt.Sprint to construct and log a message.
func Warn(args ...interface{}) {
	sugarLogger.Warn(args)
}

// Error uses fmt.Sprint to construct and log a message.
func Error(args ...interface{}) {
	sugarLogger.Error(args)
}

// DPanic uses fmt.Sprint to construct and log a message. In development, the
// logger then panics. (See DPanicLevel for details.)
func DPanic(args ...interface{}) {
	sugarLogger.DPanic(args)
}

// Panic uses fmt.Sprint to construct and log a message, then panics.
func Panic(args ...interface{}) {
	sugarLogger.Panic(args)
}

// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
func Fatal(args ...interface{}) {
	sugarLogger.Fatal(args)
}

// Debugf uses fmt.Sprintf to log a templated message.
func Debugf(template string, args ...interface{}) {
	sugarLogger.Debugf(template, args)
	//Logger.Sugar().Debugf(template, args)
}

// Infof uses fmt.Sprintf to log a templated message.
func Infof(template string, args ...interface{}) {
	sugarLogger.Infof(template, args)
}

// Warnf uses fmt.Sprintf to log a templated message.
func Warnf(template string, args ...interface{}) {
	sugarLogger.Warnf(template, args)
}

// Errorf uses fmt.Sprintf to log a templated message.
func Errorf(template string, args ...interface{}) {
	sugarLogger.Errorf(template, args)
}

// DPanicf uses fmt.Sprintf to log a templated message. In development, the
// logger then panics. (See DPanicLevel for details.)
func DPanicf(template string, args ...interface{}) {
	sugarLogger.DPanicf(template, args)
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func Panicf(template string, args ...interface{}) {
	sugarLogger.Panicf(template, args)
}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func Fatalf(template string, args ...interface{}) {
	sugarLogger.Fatalf(template, args)
}

// Debugw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
//
// When debug-level logging is disabled, this is much faster than
//  s.With(keysAndValues).Debug(msg)
func Debugw(msg string, keysAndValues ...interface{}) {
	sugarLogger.Debugw(msg, keysAndValues)
}

// Infow logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Infow(msg string, keysAndValues ...interface{}) {
	sugarLogger.Infow(msg, keysAndValues)
}

// Warnw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Warnw(msg string, keysAndValues ...interface{}) {
	sugarLogger.Warnw(msg, keysAndValues)
}

// Errorw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Errorw(msg string, keysAndValues ...interface{}) {
	sugarLogger.Errorw(msg, keysAndValues)
}

// DPanicw logs a message with some additional context. In development, the
// logger then panics. (See DPanicLevel for details.) The variadic key-value
// pairs are treated as they are in With.
func DPanicw(msg string, keysAndValues ...interface{}) {
	sugarLogger.DPanicw(msg, keysAndValues)
}

// Panicw logs a message with some additional context, then panics. The
// variadic key-value pairs are treated as they are in With.
func Panicw(msg string, keysAndValues ...interface{}) {
	sugarLogger.Panicw(msg, keysAndValues)
}

// Fatalw logs a message with some additional context, then calls os.Exit. The
// variadic key-value pairs are treated as they are in With.
func Fatalw(msg string, keysAndValues ...interface{}) {
	sugarLogger.Fatalw(msg, keysAndValues)
}
