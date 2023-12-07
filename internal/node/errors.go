package node

import "errors"

var (
	ErrDatadirUsed = errors.New("datadir already used by another process")
	ErrNodeStopped = errors.New("node not started")
	ErrNodeRunning = errors.New("node already running")
)
