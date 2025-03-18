package core

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/quka-ai/quka-ai/pkg/metrics"
)

type Metrics struct {
	apiResponseTime     *prometheus.HistogramVec
	apiErrorCounter     *prometheus.CounterVec
	chatGPTResponseTime *prometheus.HistogramVec
	chatGPTRequestTime  *prometheus.HistogramVec
	chatGPTError        *prometheus.CounterVec
	genContextTime      *prometheus.HistogramVec
}

func NewMetrics(ns, system string) *Metrics {
	// setup metric
	metrics.SetupMetricsManager(ns, system, prometheus.DefaultRegisterer.(*prometheus.Registry))

	m := &Metrics{
		apiResponseTime:     metrics.NewHistogramVec("api_response_time", []string{"api"}),
		apiErrorCounter:     metrics.NewCounterVec("api_error", []string{"method", "api", "status"}),
		chatGPTResponseTime: metrics.NewHistogramVec("chatgpt_response_time", nil),
		chatGPTRequestTime:  metrics.NewHistogramVec("chatgpt_request_time", []string{"target"}),
		chatGPTError:        metrics.NewCounterVec("chatgpt_error", []string{"type"}),
		genContextTime:      metrics.NewHistogramVec("generate_context_time", []string{"type"}),
	}

	return m
}

func (m *Metrics) ChatGPTRequestTimer(target string) *prometheus.Timer {
	return prometheus.NewTimer(m.chatGPTRequestTime.WithLabelValues(target))
}

func (m *Metrics) ApiErrorInc(method, api string, status int) {
	m.apiErrorCounter.WithLabelValues(method, api, strconv.Itoa(status)).Inc()
}

func (m *Metrics) ApiResponseTimer(api string) *prometheus.Timer {
	return prometheus.NewTimer(m.apiResponseTime.WithLabelValues(api))
}

func (m *Metrics) ChatGPTResponseTimer() *prometheus.Timer {
	return prometheus.NewTimer(m.chatGPTResponseTime.WithLabelValues())
}

func (m *Metrics) ChatGPTErrorInc(types string) {
	m.chatGPTError.WithLabelValues(types).Inc()
}

func (m *Metrics) GenContextTimer(types string) *prometheus.Timer {
	return prometheus.NewTimer(m.genContextTime.WithLabelValues(types))
}
