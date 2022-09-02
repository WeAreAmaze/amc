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

import "testing"

func TestStdLogger(t *testing.T) {
	logger := DefaultLogger
	logger = With(logger, "caller", DefaultCaller, "ts", DefaultTimestamp)

	_ = logger.Log(LevelInfo, "msg", "test debug")
	_ = logger.Log(LevelInfo, "msg", "test info")
	_ = logger.Log(LevelInfo, "msg", "test warn")
	_ = logger.Log(LevelInfo, "msg", "test error")
	_ = logger.Log(LevelDebug, "singular")

	logger2 := DefaultLogger
	_ = logger2.Log(LevelDebug)
}
