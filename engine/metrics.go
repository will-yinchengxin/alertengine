package engine

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics 监控指标
type Metrics struct {
	// 规则加载数量
	RulesLoaded *prometheus.GaugeVec

	// 告警通知发送数量
	NotificationsSent prometheus.Counter

	// 告警通知错误数量
	NotifyErrors prometheus.Counter

	// 规则重载成功次数
	ReloadSuccess prometheus.Counter

	// 规则重载失败次数
	ReloadErrors prometheus.Counter

	// 规则评估持续时间
	EvaluationDuration prometheus.Histogram

	// 活跃管理器数量
	ActiveManagers prometheus.Gauge
}

func NewMetrics() *Metrics {
	return &Metrics{
		RulesLoaded: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "alertengine_rules_loaded",
				Help: "Number of loaded alert rules per Prometheus instance",
			},
			[]string{"prom_id"},
		),
		NotificationsSent: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "alertengine_notifications_sent_total",
				Help: "Total number of alert notifications sent",
			},
		),
		NotifyErrors: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "alertengine_notify_errors_total",
				Help: "Total number of notification errors",
			},
		),
		ReloadSuccess: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "alertengine_reload_success_total",
				Help: "Total number of successful rule reloads",
			},
		),
		ReloadErrors: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "alertengine_reload_errors_total",
				Help: "Total number of rule reload errors",
			},
		),
		EvaluationDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "alertengine_evaluation_duration_seconds",
				Help:    "Duration of rule evaluation in seconds",
				Buckets: prometheus.DefBuckets,
			},
		),
		ActiveManagers: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "alertengine_active_managers",
				Help: "Number of active rule managers",
			},
		),
	}
}
