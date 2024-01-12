package infraprompushgw

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"go.uber.org/zap"

	infralog "github.com/pushwoosh/infra/log"
)

func Publish(cfg *Config, collector prometheus.Collector, jobName string) {
	if cfg == nil || !cfg.Enabled {
		return
	}

	infralog.Info("infraprompushgw: publishing metrics to push gateway")

	if jobName == "" {
		infralog.Error("infraprompushgw: job name is empty. discarding metrics")
		return
	}

	if err := push.New(cfg.Address, jobName).Collector(collector).Push(); err != nil {
		infralog.Error("infraprompushgw: publish metrics", zap.Error(err))
	}
}
