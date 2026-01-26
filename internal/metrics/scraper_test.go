package metrics

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsScraperRingBuffer(t *testing.T) {
	scraper := NewMetricsScraper(ScraperSettings{BufferSize: 3})

	scraper.recordSamples(map[string]*counterState{
		"myapp": {success: 1},
	})
	scraper.recordSamples(map[string]*counterState{
		"myapp": {success: 3},
	})
	scraper.recordSamples(map[string]*counterState{
		"myapp": {success: 6},
	})

	// Fetch returns newest to oldest
	samples := scraper.Fetch("myapp", 3)
	assert.Len(t, samples, 3)
	assert.Equal(t, int64(3), samples[0].Success) // newest
	assert.Equal(t, int64(2), samples[1].Success)
	assert.Equal(t, int64(0), samples[2].Success) // oldest

	scraper.recordSamples(map[string]*counterState{
		"myapp": {success: 10},
	})
	samples = scraper.Fetch("myapp", 3)
	assert.Equal(t, int64(4), samples[0].Success) // newest
	assert.Equal(t, int64(3), samples[1].Success)
	assert.Equal(t, int64(2), samples[2].Success) // oldest (first sample evicted)
}

func TestMetricsScraperFetchLessThanAvailable(t *testing.T) {
	scraper := NewMetricsScraper(ScraperSettings{BufferSize: 10})

	scraper.recordSamples(map[string]*counterState{"myapp": {success: 10}})
	scraper.recordSamples(map[string]*counterState{"myapp": {success: 20}})
	scraper.recordSamples(map[string]*counterState{"myapp": {success: 30}})

	// Fetch 2 returns the 2 newest
	samples := scraper.Fetch("myapp", 2)
	assert.Len(t, samples, 2)
	assert.Equal(t, int64(10), samples[0].Success) // newest
	assert.Equal(t, int64(10), samples[1].Success) // second newest
}

func TestMetricsScraperFetchMoreThanAvailable(t *testing.T) {
	scraper := NewMetricsScraper(ScraperSettings{BufferSize: 10})

	scraper.recordSamples(map[string]*counterState{"myapp": {success: 10}})
	scraper.recordSamples(map[string]*counterState{"myapp": {success: 20}})

	// Returns only available items, no padding
	samples := scraper.Fetch("myapp", 5)
	assert.Len(t, samples, 2)
	assert.Equal(t, int64(10), samples[0].Success) // newest
	assert.Equal(t, int64(0), samples[1].Success)  // second newest (first sample has 0 delta)
}

func TestMetricsScraperFetchEmpty(t *testing.T) {
	scraper := NewMetricsScraper(ScraperSettings{BufferSize: 10})

	// Returns nil for unknown service
	samples := scraper.Fetch("myapp", 5)
	assert.Nil(t, samples)
}

func TestMetricsScraperFetchUnknownService(t *testing.T) {
	scraper := NewMetricsScraper(ScraperSettings{BufferSize: 10})

	scraper.recordSamples(map[string]*counterState{"myapp": {success: 10}})

	// Returns nil for unknown service
	samples := scraper.Fetch("otherapp", 5)
	assert.Nil(t, samples)
}

func TestMetricsScraperMultipleServices(t *testing.T) {
	scraper := NewMetricsScraper(ScraperSettings{BufferSize: 10})

	scraper.recordSamples(map[string]*counterState{
		"app1": {success: 100},
		"app2": {success: 200},
	})
	scraper.recordSamples(map[string]*counterState{
		"app1": {success: 150},
		"app2": {success: 250},
	})

	samples1 := scraper.Fetch("app1", 2)
	samples2 := scraper.Fetch("app2", 2)

	// Newest first
	assert.Equal(t, int64(50), samples1[0].Success)
	assert.Equal(t, int64(50), samples2[0].Success)
}

func TestMetricsScraperDeltaCounterReset(t *testing.T) {
	scraper := NewMetricsScraper(ScraperSettings{BufferSize: 10})

	scraper.recordSamples(map[string]*counterState{"myapp": {success: 100}})
	scraper.recordSamples(map[string]*counterState{"myapp": {success: 10}})

	samples := scraper.Fetch("myapp", 2)
	// Newest first - the reset sample shows 10 (current value used as delta)
	assert.Equal(t, int64(10), samples[0].Success)
}

func TestMetricsScraperParseMetrics(t *testing.T) {
	input := `# HELP kamal_proxy_http_requests_total HTTP requests processed
# TYPE kamal_proxy_http_requests_total counter
kamal_proxy_http_requests_total{service="myapp",method="GET",status="200"} 150
kamal_proxy_http_requests_total{service="myapp",method="POST",status="201"} 50
kamal_proxy_http_requests_total{service="myapp",method="GET",status="404"} 30
kamal_proxy_http_requests_total{service="myapp",method="GET",status="500"} 10
kamal_proxy_http_requests_total{service="otherapp",method="GET",status="200"} 1000
`
	scraper := NewMetricsScraper(ScraperSettings{})
	counters, err := scraper.parseMetrics(strings.NewReader(input))

	assert.NoError(t, err)
	assert.Len(t, counters, 2)

	assert.Equal(t, float64(200), counters["myapp"].success)
	assert.Equal(t, float64(30), counters["myapp"].clientErrors)
	assert.Equal(t, float64(10), counters["myapp"].serverErrors)

	assert.Equal(t, float64(1000), counters["otherapp"].success)
}

func TestMetricsScraperParseRealData(t *testing.T) {
	input := `# HELP kamal_proxy_http_requests_total HTTP requests processed, labeled by service, status code and method.
# TYPE kamal_proxy_http_requests_total counter
kamal_proxy_http_requests_total{method="GET",service="once-campfire",status="101"} 1
kamal_proxy_http_requests_total{method="GET",service="once-campfire",status="200"} 4503
kamal_proxy_http_requests_total{method="GET",service="once-campfire",status="302"} 4401
kamal_proxy_http_requests_total{method="GET",service="once-campfire",status="304"} 411
`
	scraper := NewMetricsScraper(ScraperSettings{})
	counters, err := scraper.parseMetrics(strings.NewReader(input))

	assert.NoError(t, err)
	t.Logf("counters: %+v", counters)
	t.Logf("once-campfire: %+v", counters["once-campfire"])

	// 101 + 200 + 302 + 304 are all success (< 400)
	expectedSuccess := float64(1 + 4503 + 4401 + 411)
	assert.Equal(t, expectedSuccess, counters["once-campfire"].success)
}

func TestMetricsScraperDeltaWithRealData(t *testing.T) {
	scraper := NewMetricsScraper(ScraperSettings{BufferSize: 10})

	// First scrape - establishes baseline
	scraper.recordSamples(map[string]*counterState{
		"once-campfire": {success: 7316}, // 1 + 3503 + 3401 + 411
	})

	// Second scrape - 2000 more requests
	scraper.recordSamples(map[string]*counterState{
		"once-campfire": {success: 9316}, // 1 + 4503 + 4401 + 411
	})

	samples := scraper.Fetch("once-campfire", 2)
	t.Logf("samples[0] (newest): %+v", samples[0])
	t.Logf("samples[1] (older): %+v", samples[1])

	// The delta should be 2000
	assert.Equal(t, int64(2000), samples[0].Success)
}

func TestMetricsScraperParseMetricsEmptyInput(t *testing.T) {
	scraper := NewMetricsScraper(ScraperSettings{})
	counters, err := scraper.parseMetrics(strings.NewReader(""))

	assert.NoError(t, err)
	assert.Empty(t, counters)
}

func TestMetricsScraperSettingsDefaults(t *testing.T) {
	settings := ScraperSettings{Port: 9090}
	settings = settings.withDefaults()

	assert.Equal(t, 5_000_000_000, int(settings.Interval))
	assert.Equal(t, 200, settings.BufferSize)
}
