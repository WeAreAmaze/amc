package prometheus

import (
	"fmt"
	"github.com/amazechain/amc/log"
	"net/http"
)

var EnabledExpensive = false

// Setup starts a dedicated metrics server at the given address.
// This function enables metrics reporting separate from pprof.
func Setup(address string, log log.Logger) *http.ServeMux {
	prometheusMux := http.NewServeMux()

	prometheusMux.Handle("/debug/metrics/prometheus", Handler(DefaultRegistry))

	promServer := &http.Server{
		Addr:    address,
		Handler: prometheusMux,
	}

	go func() {
		if err := promServer.ListenAndServe(); err != nil {
			log.Error("Failure in running Prometheus server", "err", err)
		}
	}()

	log.Info("Enabling metrics export to prometheus", "path", fmt.Sprintf("http://%s/debug/metrics/prometheus", address))

	return prometheusMux
}
