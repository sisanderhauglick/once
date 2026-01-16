package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"

	prom "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

// Sample represents request counts for a single scrape interval
type Sample struct {
	Timestamp    time.Time
	Success      int64
	ClientErrors int64
	ServerErrors int64
}

// ScraperSettings configures the metrics scraper
type ScraperSettings struct {
	Port       int
	Interval   time.Duration
	BufferSize int
}

func (s ScraperSettings) withDefaults() ScraperSettings {
	if s.Interval == 0 {
		s.Interval = 5 * time.Second
	}
	if s.BufferSize == 0 {
		s.BufferSize = 200
	}
	return s
}

// MetricsScraper periodically scrapes Prometheus metrics from kamal-proxy
type MetricsScraper struct {
	settings ScraperSettings
	client   *http.Client

	mu        sync.RWMutex
	services  map[string]*serviceData
	lastError error

	cancel context.CancelFunc
	done   chan struct{}
}

type serviceData struct {
	samples      []Sample
	head         int
	count        int
	prevCounters *counterState
}

type counterState struct {
	success      float64
	clientErrors float64
	serverErrors float64
}

func NewMetricsScraper(settings ScraperSettings) *MetricsScraper {
	settings = settings.withDefaults()
	return &MetricsScraper{
		settings: settings,
		client:   &http.Client{Timeout: 5 * time.Second},
		services: make(map[string]*serviceData),
	}
}

func (s *MetricsScraper) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	s.done = make(chan struct{})

	go s.run(ctx)
}

func (s *MetricsScraper) Stop() {
	if s.cancel != nil {
		s.cancel()
		<-s.done
	}
}

func (s *MetricsScraper) Services() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	services := make([]string, 0, len(s.services))
	for name := range s.services {
		services = append(services, name)
	}
	slices.Sort(services)
	return services
}

// Fetch returns the last n samples for a service, ordered from newest to oldest.
// If fewer than n samples exist, only the available samples are returned.
func (s *MetricsScraper) Fetch(service string, n int) []Sample {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.services[service]
	if !ok {
		return nil
	}

	available := min(n, data.count)
	result := make([]Sample, available)
	for i := range available {
		idx := (data.head - 1 - i + len(data.samples)) % len(data.samples)
		result[i] = data.samples[idx]
	}

	return result
}

// FetchAverage returns a moving sum over a sliding window.
// Each of the `points` results is the sum of `window` consecutive samples,
// scaled up when insufficient data exists to fill the window.
func (s *MetricsScraper) FetchAverage(service string, points, window int) []Sample {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Sample, points)

	data, ok := s.services[service]
	if !ok || data.count == 0 {
		return result
	}

	for i := range points {
		var sum Sample
		available := 0

		for j := range window {
			sampleIdx := i + j
			if sampleIdx >= data.count {
				break
			}
			idx := (data.head - 1 - sampleIdx + len(data.samples)) % len(data.samples)
			sample := data.samples[idx]
			sum.Success += sample.Success
			sum.ClientErrors += sample.ClientErrors
			sum.ServerErrors += sample.ServerErrors
			available++
		}

		if available > 0 {
			scale := int64(window) / int64(available)
			result[i] = Sample{
				Success:      sum.Success * scale,
				ClientErrors: sum.ClientErrors * scale,
				ServerErrors: sum.ServerErrors * scale,
			}
		}
	}

	return result
}

func (s *MetricsScraper) Latest(service string) (Sample, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.services[service]
	if !ok || data.count == 0 {
		return Sample{}, false
	}

	idx := (data.head - 1 + len(data.samples)) % len(data.samples)
	return data.samples[idx], true
}

func (s *MetricsScraper) LastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// Private

func (s *MetricsScraper) run(ctx context.Context) {
	defer close(s.done)

	ticker := time.NewTicker(s.settings.Interval)
	defer ticker.Stop()

	s.scrape(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scrape(ctx)
		}
	}
}

func (s *MetricsScraper) scrape(ctx context.Context) {
	url := fmt.Sprintf("http://127.0.0.1:%d/metrics", s.settings.Port)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		s.setError(fmt.Errorf("creating request: %w", err))
		return
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.setError(fmt.Errorf("fetching metrics: %w", err))
		return
	}
	defer resp.Body.Close()

	counters, err := s.parseMetrics(resp.Body)
	if err != nil {
		s.setError(fmt.Errorf("parsing metrics: %w", err))
		return
	}

	s.setError(nil)
	s.recordSamples(counters)
}

func (s *MetricsScraper) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastError = err
}

func (s *MetricsScraper) parseMetrics(body io.Reader) (map[string]*counterState, error) {
	parser := expfmt.NewTextParser(model.UTF8Validation)
	families, err := parser.TextToMetricFamilies(body)
	if err != nil {
		return nil, err
	}

	counters := make(map[string]*counterState)

	family, ok := families["kamal_proxy_http_requests_total"]
	if !ok {
		return counters, nil
	}

	for _, metric := range family.GetMetric() {
		service := getLabel(metric, "service")
		if service == "" {
			continue
		}

		state, ok := counters[service]
		if !ok {
			state = &counterState{}
			counters[service] = state
		}

		statusCode := getStatusCode(metric)
		count := metric.GetCounter().GetValue()

		switch {
		case statusCode >= 100 && statusCode < 400:
			state.success += count
		case statusCode >= 400 && statusCode < 500:
			state.clientErrors += count
		case statusCode >= 500:
			state.serverErrors += count
		}
	}

	return counters, nil
}

func (s *MetricsScraper) recordSamples(counters map[string]*counterState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	for service, current := range counters {
		data, ok := s.services[service]
		if !ok {
			data = &serviceData{
				samples: make([]Sample, s.settings.BufferSize),
			}
			s.services[service] = data
		}

		sample := Sample{Timestamp: now}
		if data.prevCounters != nil {
			sample.Success = safeDelta(current.success, data.prevCounters.success)
			sample.ClientErrors = safeDelta(current.clientErrors, data.prevCounters.clientErrors)
			sample.ServerErrors = safeDelta(current.serverErrors, data.prevCounters.serverErrors)
		}
		data.prevCounters = current

		data.samples[data.head] = sample
		data.head = (data.head + 1) % len(data.samples)
		if data.count < len(data.samples) {
			data.count++
		}
	}
}

// Helpers

func getLabel(metric *prom.Metric, name string) string {
	for _, label := range metric.GetLabel() {
		if label.GetName() == name {
			return label.GetValue()
		}
	}
	return ""
}

func getStatusCode(metric *prom.Metric) int {
	status := getLabel(metric, "status")
	var code int
	fmt.Sscanf(status, "%d", &code)
	return code
}

func safeDelta(current, prev float64) int64 {
	if current < prev {
		return int64(current)
	}
	return int64(current - prev)
}
