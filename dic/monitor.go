package dic

import (
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/help"
)

var monitorLogger = help.GetLogger("org.wltea.analyzer.dic.Monitor")

// monitorClient carries the timeouts the original set on its HTTP HEAD: ten
// seconds to obtain a connection, ten to establish it, fifteen on the socket.
var monitorClient = &http.Client{
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		ResponseHeaderTimeout: 15 * time.Second,
		DisableCompression:    true,
	},
}

// Monitor polls one remote dictionary and reloads the live dictionary whenever
// the server reports the resource has changed.
type Monitor struct {
	lastModified string
	eTags        string
	location     string

	configuration cfg.Configuration
}

// NewMonitor returns a monitor for one remote dictionary location.
func NewMonitor(location string, configuration cfg.Configuration) *Monitor {
	return &Monitor{location: location, configuration: configuration}
}

// Run performs one poll.
func (m *Monitor) Run() {
	cfg.Check(m.configuration)
	m.runUnprivileged()
}

// runUnprivileged sends a conditional HEAD, and reloads the dictionary when the
// server answers 200 with a Last-Modified or ETag that differs from the one
// carried over from the previous poll. 304 means unchanged; anything else is
// logged and ignored.
func (m *Monitor) runUnprivileged() {
	request, err := http.NewRequest(http.MethodHead, m.location, nil)
	if err != nil {
		monitorLogger.Error("remote_ext_dict {} error!", err, m.location)
		return
	}
	request.Header.Set("Accept-Encoding", acceptEncoding)
	if m.lastModified != "" {
		request.Header.Set("If-Modified-Since", m.lastModified)
	}
	if m.eTags != "" {
		request.Header.Set("If-None-Match", m.eTags)
	}

	response, err := monitorClient.Do(request)
	if err != nil {
		monitorLogger.Error("remote_ext_dict {} error!", err, m.location)
		return
	}
	defer func() { _, _ = io.Copy(io.Discard, response.Body); _ = response.Body.Close() }()

	switch response.StatusCode {
	case http.StatusOK:
		lastModified := lastHeader(response.Header, "Last-Modified")
		eTag := lastHeader(response.Header, "ETag")
		if (lastModified != "" && !strings.EqualFold(lastModified, m.lastModified)) ||
			(eTag != "" && !strings.EqualFold(eTag, m.eTags)) {
			GetSingleton().reLoadMainDict()
			m.lastModified = lastModified
			m.eTags = eTag
		}
	case http.StatusNotModified:
		// Unchanged; nothing to do.
	default:
		monitorLogger.Info("remote_ext_dict {} return bad code {}", m.location, response.StatusCode)
	}
}

// lastHeader returns the last value of a repeated header. The original reads
// the last one; Header.Get would return the first, so a server that repeats
// Last-Modified would be compared against the wrong value.
func lastHeader(header http.Header, name string) string {
	values := header.Values(name)
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

// scheduler runs every registered monitor on a single worker, which is what a
// one-thread scheduled executor gave the original: polls never overlap.
type scheduler struct {
	mu    sync.Mutex
	tasks []*Monitor
	// started is set once the worker goroutine is running.
	started bool
	delay   time.Duration
	period  time.Duration
}

func (s *scheduler) scheduleAtFixedRate(m *Monitor, delay, period time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks = append(s.tasks, m)
	if s.started {
		return
	}
	s.started, s.delay, s.period = true, delay, period
	go s.loop()
}

func (s *scheduler) loop() {
	time.Sleep(s.delay)
	ticker := time.NewTicker(s.period)
	defer ticker.Stop()
	for {
		s.mu.Lock()
		tasks := append([]*Monitor(nil), s.tasks...)
		s.mu.Unlock()
		for _, task := range tasks {
			task.Run()
		}
		<-ticker.C
	}
}
