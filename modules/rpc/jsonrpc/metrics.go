package jsonrpc

import (
	"fmt"
	"github.com/amazechain/amc/internal/metrics/prometheus"
)

var (
	rpcMetricsLabels   = map[bool]map[string]string{}
	rpcRequestGauge    = prometheus.GetOrCreateCounter("rpc_total")
	failedReqeustGauge = prometheus.GetOrCreateCounter("rpc_failure")
)

func createRPCMetricsLabel(method string, valid bool) string {
	status := "failure"
	if valid {
		status = "success"
	}

	return fmt.Sprintf(`rpc_duration_seconds{method="%s",success="%s"}`, method, status)

}

func newRPCServingTimerMS(method string, valid bool) prometheus.Summary {
	label, ok := rpcMetricsLabels[valid][method]
	if !ok {
		label = createRPCMetricsLabel(method, valid)
	}

	return prometheus.GetOrCreateSummary(label)
}
