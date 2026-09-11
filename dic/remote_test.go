package dic

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/jdk"
)

const remoteWordList = "远程热词\n量子纠缠态\n  空白包围词  \n\nGoLang\n"

func TestGetRemoteWordsReadsALengthedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		_, _ = w.Write([]byte(remoteWordList))
	}))
	defer server.Close()

	configuration := &testConfiguration{Settings: cfg.NewSettings()}
	words := getRemoteWords(server.URL, configuration)
	if configuration.checked != 1 {
		t.Errorf("the permission hook ran %d times, want 1", configuration.checked)
	}
	want := []string{"远程热词", "量子纠缠态", "  空白包围词  ", "", "GoLang"}
	if len(words) != len(want) {
		t.Fatalf("words = %q, want %q", words, want)
	}
	for i := range want {
		if words[i] != want[i] {
			t.Errorf("word %d = %q, want %q — lines arrive untrimmed", i, words[i], want[i])
		}
	}
}

func TestGetRemoteWordsReadsAChunkedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		flusher := w.(http.Flusher)
		for _, part := range []string{"远程热词\n", "量子纠缠态\n"} {
			_, _ = w.Write([]byte(part))
			flusher.Flush()
		}
	}))
	defer server.Close()

	if got := len(getRemoteWords(server.URL, &testConfiguration{Settings: cfg.NewSettings()})); got != 2 {
		t.Errorf("read %d words from a chunked body, want 2", got)
	}
}

// A gzipped dictionary is silently skipped. Decompression replaces the entity
// with one that reports no length, and a body of unknown length that is not
// chunked is never read. Reproduced deliberately: repairing it here would load
// words the original never loaded.
func TestGetRemoteWordsSkipsAGzippedBody(t *testing.T) {
	var served bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept-Encoding"); got != acceptEncoding {
			t.Errorf("Accept-Encoding = %q, want %q", got, acceptEncoding)
		}
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write([]byte(remoteWordList))
		_ = zw.Close()

		served = true
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(buf.Bytes())
	}))
	defer server.Close()

	if got := getRemoteWords(server.URL, &testConfiguration{Settings: cfg.NewSettings()}); len(got) != 0 {
		t.Errorf("words = %q, want none", got)
	}
	if !served {
		t.Error("the server was never reached")
	}
}

func TestGetRemoteWordsIgnoresEverythingButTwoHundred(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer server.Close()

	if got := getRemoteWords(server.URL, &testConfiguration{Settings: cfg.NewSettings()}); len(got) != 0 {
		t.Errorf("words = %q, want none", got)
	}
	if got := getRemoteWords("http://127.0.0.1:1/unreachable", &testConfiguration{Settings: cfg.NewSettings()}); len(got) != 0 {
		t.Errorf("words = %q, want none", got)
	}
	if got := getRemoteWords("://not a url", &testConfiguration{Settings: cfg.NewSettings()}); len(got) != 0 {
		t.Errorf("words = %q, want none", got)
	}
}

func TestGetRemoteWordsHonoursTheDeclaredCharset(t *testing.T) {
	serve := func(contentType string, body []byte) []string {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			_, _ = w.Write(body)
		}))
		defer server.Close()
		return getRemoteWords(server.URL, &testConfiguration{Settings: cfg.NewSettings()})
	}

	if got := serve("text/plain; charset=ISO-8859-1", []byte{0xE9, 0x74, 0xE9, '\n'}); len(got) != 1 || got[0] != "été" {
		t.Errorf("latin-1 body decoded to %q", got)
	}
	if got := serve("text/plain", []byte("默认UTF8\n")); len(got) != 1 || got[0] != "默认UTF8" {
		t.Errorf("default charset decoded to %q", got)
	}
	// A charset with no Go decoder is reported and the words are dropped, the
	// way an unsupported encoding was in the original.
	if got := serve("text/plain; charset=Shift_JIS", []byte("abc\n")); len(got) != 0 {
		t.Errorf("unsupported charset yielded %q, want none", got)
	}
}

func remoteDictionary(t *testing.T, location string) *Dictionary {
	t.Helper()
	dir := writeConfigDir(t, map[string]string{
		pathDicMain: "中华\n",
		fileName: `<?xml version="1.0" encoding="UTF-8"?>
<properties><entry key="remote_ext_dict">` + location + `</entry></properties>`,
	})
	return newTestDictionary(t, dir)
}

// The monitor reloads only when the server reports a change, and a conditional
// request that comes back 304 leaves the dictionary alone.
func TestMonitorReloadsOnlyWhenTheResourceChanges(t *testing.T) {
	var etag = "\"v1\""
	var body = "量子纠缠态\n"
	var heads, gets int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			heads++
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", etag)
			w.Header().Set("Last-Modified", "Wed, 01 Jan 2020 00:00:00 GMT")
			w.WriteHeader(http.StatusOK)
			return
		}
		gets++
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	d := remoteDictionary(t, server.URL)
	withSingleton(t, d)
	if !d.MatchAllInMainDict(jdk.EncodeUTF16("量子纠缠态")).IsMatch() {
		t.Fatal("the remote dictionary should load at startup")
	}

	c := &testConfiguration{Settings: cfg.NewSettings()}
	monitor := NewMonitor(server.URL, c)

	monitor.Run()
	if c.checked != 1 {
		t.Errorf("the permission hook ran %d times, want 1", c.checked)
	}
	if monitor.eTags != etag {
		t.Errorf("eTags = %q, want %q", monitor.eTags, etag)
	}
	firstGets := gets

	// Nothing changed, so the conditional HEAD comes back 304 and no reload runs.
	monitor.Run()
	if gets != firstGets {
		t.Errorf("a 304 should not refetch the dictionary (%d -> %d)", firstGets, gets)
	}

	// A new ETag brings in the new words.
	etag = "\"v2\""
	body = "深度学习框架\n"
	monitor.Run()
	if !d.MatchAllInMainDict(jdk.EncodeUTF16("深度学习框架")).IsMatch() {
		t.Error("the changed dictionary should have been reloaded")
	}
	if heads != 3 {
		t.Errorf("issued %d HEAD requests, want 3", heads)
	}
}

func TestMonitorToleratesBadStatusesAndUnreachableServers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	d := newTestDictionary(t, writeConfigDir(t, map[string]string{pathDicMain: "中华\n"}))
	withSingleton(t, d)

	c := &testConfiguration{Settings: cfg.NewSettings()}
	NewMonitor(server.URL, c).Run()
	NewMonitor("http://127.0.0.1:1/unreachable", c).Run()
	NewMonitor("://not a url", c).Run()

	if !d.MatchAllInMainDict(jdk.EncodeUTF16("中华")).IsMatch() {
		t.Error("a failed poll must leave the dictionary alone")
	}
}

func TestLastHeaderTakesTheFinalValue(t *testing.T) {
	header := http.Header{}
	header.Add("ETag", "\"first\"")
	header.Add("ETag", "\"last\"")
	if got, want := lastHeader(header, "ETag"), "\"last\""; got != want {
		t.Errorf("lastHeader = %q, want %q", got, want)
	}
	if got := lastHeader(header, "Absent"); got != "" {
		t.Errorf("lastHeader = %q, want empty", got)
	}
}

// The monitors share one worker, so polls never overlap however many remote
// dictionaries are configured.
func TestSchedulerRunsEveryMonitorOnOneWorker(t *testing.T) {
	polls := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		polls <- r.URL.Path
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	d := newTestDictionary(t, writeConfigDir(t, map[string]string{pathDicMain: "中华\n"}))
	withSingleton(t, d)

	c := &testConfiguration{Settings: cfg.NewSettings()}
	var s scheduler
	s.scheduleAtFixedRate(NewMonitor(server.URL+"/one", c), time.Millisecond, time.Hour)
	s.scheduleAtFixedRate(NewMonitor(server.URL+"/two", c), time.Millisecond, time.Hour)

	seen := map[string]bool{}
	for range 2 {
		select {
		case path := <-polls:
			seen[path] = true
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out; saw %v", seen)
		}
	}
	if !seen["/one"] || !seen["/two"] {
		t.Errorf("polled %v, want both locations", seen)
	}
}
