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

package log

import (
	"context"
	"log"
)

// DefaultLogger is default logger.
var DefaultLogger = NewStdLogger(log.Writer())

// Logger is a logger interface.
type Logger interface {
	Log(level Level, keyvals ...interface{}) error
}

type logger struct {
	logger    Logger
	prefix    []interface{}
	hasValuer bool
	ctx       context.Context
}

func (c *logger) Log(level Level, keyvals ...interface{}) error {
	kvs := make([]interface{}, 0, len(c.prefix)+len(keyvals))
	kvs = append(kvs, c.prefix...)
	if c.hasValuer {
		bindValues(c.ctx, kvs)
	}
	kvs = append(kvs, keyvals...)
	if err := c.logger.Log(level, kvs...); err != nil {
		return err
	}
	return nil
}

// With with logger fields.
func With(l Logger, kv ...interface{}) Logger {
	c, ok := l.(*logger)
	if !ok {
		return &logger{logger: l, prefix: kv, hasValuer: containsValuer(kv), ctx: context.Background()}
	}
	kvs := make([]interface{}, 0, len(c.prefix)+len(kv))
	kvs = append(kvs, kv...)
	kvs = append(kvs, c.prefix...)
	return &logger{
		logger:    c.logger,
		prefix:    kvs,
		hasValuer: containsValuer(kvs),
		ctx:       c.ctx,
	}
}

// WithContext returns a shallow copy of l with its context changed
// to ctx. The provided ctx must be non-nil.
func WithContext(ctx context.Context, l Logger) Logger {
	c, ok := l.(*logger)
	if !ok {
		return &logger{logger: l, ctx: ctx}
	}
	return &logger{
		logger:    c.logger,
		prefix:    c.prefix,
		hasValuer: c.hasValuer,
		ctx:       ctx,
	}
}
