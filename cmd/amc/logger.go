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

package main

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
	once sync.Once
)

func Init(nodeConfig *conf.NodeConfig, config *conf.LoggerConfig) (*zap.Logger, error) {

	var level zapcore.Level
	if err := level.Set(config.Level); err != nil {
		return nil, err
	}

	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= level && lvl < zapcore.ErrorLevel
	})

	errorLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})

	var Logger *zap.Logger

	once.Do(func() {
		encode := getEncoder()
		core := zapcore.NewTee(
			zapcore.NewCore(encode, getLogWriter(nodeConfig, config), infoLevel),
			zapcore.NewCore(encode, zapcore.AddSync(os.Stdout), level),
			zapcore.NewCore(encode, getErrorWriter(nodeConfig, config), errorLevel),
		)

		Logger = zap.New(core, zap.AddCaller())
	})

	return Logger, nil
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeName = zapcore.FullNameEncoder
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

func getLogWriter(nodeConfig *conf.NodeConfig, config *conf.LoggerConfig) zapcore.WriteSyncer {
	return zapcore.AddSync(getLumberjack(fmt.Sprintf("%s/log/%s", nodeConfig.DataDir, config.LogFile), config.MaxSize, config.MaxBackups, config.MaxAge, config.Compress))
}

func getErrorWriter(nodeConfig *conf.NodeConfig, config *conf.LoggerConfig) zapcore.WriteSyncer {
	var name string
	i := strings.LastIndex(config.LogFile, ".")
	if i <= 0 {
		name = fmt.Sprintf("%s_error.log", config.LogFile)
	} else {
		name = fmt.Sprintf("%s_error.log", config.LogFile[:i])
	}
	return zapcore.AddSync(getLumberjack(fmt.Sprintf("%s/log/%s", nodeConfig.DataDir, name), config.MaxSize, config.MaxBackups, config.MaxAge, config.Compress))
}
