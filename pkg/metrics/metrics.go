package metrics

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type manager struct {
	namespace string
	system    string
	registry  *prometheus.Registry
}

const defaultKey = "default"

var defaultManager map[string]*manager

func init() {
	defaultManager = make(map[string]*manager)
	defaultManager[defaultKey] = &manager{
		namespace: defaultKey,
		system:    defaultKey,
		registry:  prometheus.NewRegistry(), // ignore registry
	}
}

func RegisterGoMetrics(r prometheus.Registerer) {
	r.Register(collectors.NewGoCollector())
}

func SetupMetricsManager(ns, system string, registry *prometheus.Registry) {
	defaultManager[defaultKey] = &manager{
		namespace: ns,
		system:    system,
		registry:  registry,
	}
	RegisterGoMetrics(registry)
}

func MustGetDefaultManager() (string, string, prometheus.Registerer) {
	m := defaultManager[defaultKey]
	return m.namespace, m.system, m.registry
}

func NewCounterVec(name string, labels []string) *prometheus.CounterVec {
	ns, system, registerer := MustGetDefaultManager()

	vec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: FmtFixer(ns),
			Subsystem: FmtFixer(system),
			Name:      FmtFixer(name),
			Help:      fmt.Sprintf("%s count of /%s/%s", name, ns, system),
		},
		labels,
	)

	var initLabelValue []string
	for i := 0; i < len(labels); i++ {
		initLabelValue = append(initLabelValue, "")
	}
	vec.WithLabelValues(initLabelValue...).Add(0)

	registerer.Register(vec)
	return vec
}

func NewHistogramVec(name string, labels []string) *prometheus.HistogramVec {
	ns, system, registerer := MustGetDefaultManager()
	vec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: FmtFixer(ns),
			Subsystem: FmtFixer(system),
			Name:      FmtFixer(name),
			Help:      fmt.Sprintf("%s duration of /%s/%s", name, ns, system),
		},
		labels,
	)

	var initLabelValue []string
	for i := 0; i < len(labels); i++ {
		initLabelValue = append(initLabelValue, "")
	}
	vec.WithLabelValues(initLabelValue...).Observe(0)

	registerer.Register(vec)
	return vec
}

func NewGaugeVec(name string, labels []string) *prometheus.GaugeVec {
	ns, system, registerer := MustGetDefaultManager()

	vec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: FmtFixer(ns),
			Subsystem: FmtFixer(system),
			Name:      FmtFixer(name),
			Help:      fmt.Sprintf("%s gauge of /%s/%s", name, ns, system),
		},
		labels,
	)

	var initLabelValue []string
	for i := 0; i < len(labels); i++ {
		initLabelValue = append(initLabelValue, "")
	}
	vec.WithLabelValues(initLabelValue...).Add(0)

	registerer.Register(vec)

	return vec
}

func DefaultExportHandler() gin.HandlerFunc {
	h := promhttp.InstrumentMetricHandler(
		defaultManager[defaultKey].registry, promhttp.HandlerFor(defaultManager[defaultKey].registry, promhttp.HandlerOpts{}),
	)
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func FmtFixer(in string) string {
	return strings.Replace(strings.Replace(in, ".", "_", -1), "-", "_", -1)
}
