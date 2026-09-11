package dic

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/help"
	"github.com/infinilabs/analysis-ik/jdk"
)

// Dictionary owns the three tries the analyzer queries — the main word list,
// the quantifier list and the stop-word list — and the configuration that says
// where they live. It is a process-wide singleton, built once by Initial.
type Dictionary struct {
	mu             sync.RWMutex
	mainDict       *dictSegment
	quantifierDict *dictSegment
	stopWords      *dictSegment

	configuration cfg.Configuration
	confDir       string
	props         jdk.Properties
}

var (
	singleton   atomic.Pointer[Dictionary]
	singletonMu sync.Mutex
	monitors    scheduler
)

var logger = help.GetLogger("org.wltea.analyzer.dic.Dictionary")

// The dictionary file names, resolved against the configuration directory.
const (
	pathDicMain       = "main.dic"
	pathDicSurname    = "surname.dic"
	pathDicQuantifier = "quantifier.dic"
	pathDicSuffix     = "suffix.dic"
	pathDicPrep       = "preposition.dic"
	pathDicStop       = "stopword.dic"
)

// The configuration file and the keys it may carry.
const (
	fileName         = "IKAnalyzer.cfg.xml"
	extDict          = "ext_dict"
	remoteExtDict    = "remote_ext_dict"
	extStop          = "ext_stopwords"
	remoteExtStop    = "remote_ext_stopwords"
	monitorInitDelay = 10 * time.Second
	monitorPeriod    = 60 * time.Second
)

func newDictionary(configuration cfg.Configuration) *Dictionary {
	d := &Dictionary{
		configuration: configuration,
		confDir:       configuration.ConfDir(),
		props:         jdk.Properties{},
	}

	configFile := filepath.Join(d.confDir, fileName)
	logger.Info("try load config from {}", configFile)
	input, err := openRegularFile(configFile)
	if err != nil {
		// Fall back to the config directory shipped inside the plugin, and keep
		// looking dictionaries up there too.
		d.confDir = configuration.ConfigInPluginDir()
		configFile = filepath.Join(d.confDir, fileName)
		logger.Info("try load config from {}", configFile)
		var fallbackErr error
		if input, fallbackErr = openRegularFile(configFile); fallbackErr != nil {
			// Report the original failure, not the fallback's.
			logger.Error("ik-analyzer", err)
		}
	}
	if input != nil {
		defer func() { _ = input.Close() }()
		props, err := jdk.LoadFromXML(input)
		if err != nil {
			logger.Error("ik-analyzer", err)
		} else {
			d.props = props
		}
	}
	return d
}

func (d *Dictionary) getProperty(key string) (string, bool) {
	if d.props == nil {
		return "", false
	}
	return d.props.Get(key)
}

// Initial builds the singleton and loads every dictionary. Calls after the
// first are ignored, so the configuration that wins is the one that arrives
// first.
func Initial(configuration cfg.Configuration) {
	if singleton.Load() != nil {
		return
	}
	singletonMu.Lock()
	defer singletonMu.Unlock()
	if singleton.Load() != nil {
		return
	}

	d := newDictionary(configuration)
	d.loadMainDict()
	d.loadSurnameDict()
	d.loadQuantifierDict()
	d.loadSuffixDict()
	d.loadPrepDict()
	d.loadStopWordDict()
	singleton.Store(d)

	if configuration.EnableRemoteDict() {
		for _, location := range d.getRemoteExtDictionarys() {
			monitors.scheduleAtFixedRate(NewMonitor(location, configuration), monitorInitDelay, monitorPeriod)
		}
		for _, location := range d.getRemoteExtStopWordDictionarys() {
			monitors.scheduleAtFixedRate(NewMonitor(location, configuration), monitorInitDelay, monitorPeriod)
		}
	}
}

// GetSingleton returns the dictionary, panicking when Initial has not run —
// the analyzer cannot segment anything without one.
func GetSingleton() *Dictionary {
	d := singleton.Load()
	if d == nil {
		panic("ik dict has not been initialized yet, please call initial method first.")
	}
	return d
}

func (d *Dictionary) walkFileTree(files *[]string, path string) {
	info, err := os.Stat(path)
	switch {
	case err == nil && info.Mode().IsRegular():
		*files = append(*files, path)
	case err == nil && info.IsDir():
		walkErr := filepath.WalkDir(path, func(p string, entry fs.DirEntry, err error) error {
			if err != nil {
				logger.Error("[Ext Loading] listing files", err)
				return nil
			}
			if !entry.IsDir() {
				*files = append(*files, p)
			}
			return nil
		})
		if walkErr != nil {
			logger.Error("[Ext Loading] listing files", walkErr)
		}
	default:
		logger.Warn("[Ext Loading] file not found: " + path)
	}
}

// loadDictFile reads one dictionary file into dict. A missing critical file is
// fatal; a missing optional one is only logged.
func (d *Dictionary) loadDictFile(dict *dictSegment, file string, critical bool, name string) {
	f, err := openRegularFile(file)
	if err != nil {
		logger.Error("ik-analyzer: "+name+" not found", err)
		if critical {
			panic(fmt.Sprintf("ik-analyzer: %s not found!!!", name))
		}
		return
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 512), 1<<20)
	scanner.Split(scanJavaLines)

	first := true
	for scanner.Scan() {
		word := scanner.Text()
		if first {
			// Only the first line can carry the byte order mark.
			word = strings.TrimPrefix(word, "\uFEFF")
			first = false
		}
		word = jdk.Trim(word)
		if word == "" {
			continue
		}
		dict.fillSegmentAll(jdk.EncodeUTF16(word))
	}
	if err := scanner.Err(); err != nil {
		logger.Error("ik-analyzer: "+name+" loading failed", err)
	}
}

// openRegularFile opens a file for reading, refusing a directory. FileInputStream
// reports a directory as not-found and the loader's fallback path depends on
// that; os.Open opens one happily and only fails on the first read.
func openRegularFile(name string) (*os.File, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, &fs.PathError{Op: "open", Path: name, Err: syscall.EISDIR}
	}
	return f, nil
}

// scanJavaLines splits on CR, LF or CRLF, the way BufferedReader.readLine does.
func scanJavaLines(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '\n':
			return i + 1, data[:i], nil
		case '\r':
			if i+1 < len(data) {
				if data[i+1] == '\n' {
					return i + 2, data[:i], nil
				}
				return i + 1, data[:i], nil
			}
			if atEOF {
				return i + 1, data[:i], nil
			}
			return 0, nil, nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func (d *Dictionary) splitConfiguredPaths(key string) []string {
	value, ok := d.getProperty(key)
	if !ok {
		return nil
	}
	var out []string
	for _, filePath := range strings.Split(value, ";") {
		if jdk.Trim(filePath) != "" {
			out = append(out, jdk.Trim(filePath))
		}
	}
	return out
}

func (d *Dictionary) getExtDictionarys() []string {
	extDictFiles := make([]string, 0, 2)
	for _, filePath := range d.splitConfiguredPaths(extDict) {
		d.walkFileTree(&extDictFiles, d.configuration.Path(d.getDictRoot(), filePath))
	}
	return extDictFiles
}

func (d *Dictionary) getRemoteExtDictionarys() []string {
	return d.splitConfiguredPaths(remoteExtDict)
}

func (d *Dictionary) getExtStopWordDictionarys() []string {
	extStopWordDictFiles := make([]string, 0, 2)
	for _, filePath := range d.splitConfiguredPaths(extStop) {
		d.walkFileTree(&extStopWordDictFiles, d.configuration.Path(d.getDictRoot(), filePath))
	}
	return extStopWordDictFiles
}

func (d *Dictionary) getRemoteExtStopWordDictionarys() []string {
	return d.splitConfiguredPaths(remoteExtStop)
}

func (d *Dictionary) getDictRoot() string {
	if abs, err := filepath.Abs(d.confDir); err == nil {
		return abs
	}
	return d.confDir
}

// AddWords loads extra words into the main dictionary at runtime.
func (d *Dictionary) AddWords(words []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, word := range words {
		d.mainDict.fillSegmentAll(jdk.EncodeUTF16(jdk.Trim(word)))
	}
}

// DisableWords hides words from the main dictionary without removing their
// nodes, so a longer word sharing the prefix keeps working.
func (d *Dictionary) DisableWords(words []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, word := range words {
		d.mainDict.disableSegment(jdk.EncodeUTF16(jdk.Trim(word)))
	}
}

// AddStopWords loads extra stop words at runtime, the counterpart of AddWords
// for the stop-word trie.
func (d *Dictionary) AddStopWords(words []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, word := range words {
		d.stopWords.fillSegmentAll(jdk.EncodeUTF16(jdk.Trim(word)))
	}
}

// MatchAllInMainDict matches the whole slice against the main dictionary.
func (d *Dictionary) MatchAllInMainDict(charArray []jdk.Char) *Hit {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.mainDict.matchAll(charArray)
}

// MatchInMainDict matches length code units from begin against the main
// dictionary.
func (d *Dictionary) MatchInMainDict(charArray []jdk.Char, begin, length int) *Hit {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.mainDict.match(charArray, begin, length, nil)
}

// MatchInQuantifierDict matches length code units from begin against the
// quantifier dictionary.
func (d *Dictionary) MatchInQuantifierDict(charArray []jdk.Char, begin, length int) *Hit {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.quantifierDict.match(charArray, begin, length, nil)
}

// MatchWithHit continues a partial match from the node the hit stopped at.
func (d *Dictionary) MatchWithHit(charArray []jdk.Char, currentIndex int, matchedHit *Hit) *Hit {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return matchedHit.matchedDictSegment.match(charArray, currentIndex, 1, matchedHit)
}

// IsStopWord reports whether the span is a stop word.
func (d *Dictionary) IsStopWord(charArray []jdk.Char, begin, length int) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.stopWords.match(charArray, begin, length, nil).IsMatch()
}

func (d *Dictionary) loadMainDict() {
	d.mainDict = newDictSegment(0)
	d.loadDictFile(d.mainDict, d.configuration.Path(d.getDictRoot(), pathDicMain), false, "Main Dict")
	d.loadExtDict()
	d.loadRemoteExtDict()
}

func (d *Dictionary) loadExtDict() {
	for _, extDictName := range d.getExtDictionarys() {
		logger.Info("[Dict Loading] " + extDictName)
		d.loadDictFile(d.mainDict, d.configuration.Path(extDictName), false, "Extra Dict")
	}
}

func (d *Dictionary) loadRemoteExtDict() {
	for _, location := range d.getRemoteExtDictionarys() {
		logger.Info("[Dict Loading] " + location)
		lists := getRemoteWords(location, d.configuration)
		if lists == nil {
			logger.Error("[Dict Loading] " + location + " load failed")
			continue
		}
		for _, theWord := range lists {
			if jdk.Trim(theWord) != "" {
				logger.Info(theWord)
				d.mainDict.fillSegmentAll(jdk.EncodeUTF16(jdk.ToLower(jdk.Trim(theWord))))
			}
		}
	}
}

func (d *Dictionary) loadStopWordDict() {
	d.stopWords = newDictSegment(0)
	d.loadDictFile(d.stopWords, d.configuration.Path(d.getDictRoot(), pathDicStop), false, "Main Stopwords")

	for _, extStopWordDictName := range d.getExtStopWordDictionarys() {
		logger.Info("[Dict Loading] " + extStopWordDictName)
		d.loadDictFile(d.stopWords, d.configuration.Path(extStopWordDictName), false, "Extra Stopwords")
	}

	for _, location := range d.getRemoteExtStopWordDictionarys() {
		logger.Info("[Dict Loading] " + location)
		lists := getRemoteWords(location, d.configuration)
		if lists == nil {
			logger.Error("[Dict Loading] " + location + " load failed")
			continue
		}
		for _, theWord := range lists {
			if jdk.Trim(theWord) != "" {
				logger.Info(theWord)
				d.stopWords.fillSegmentAll(jdk.EncodeUTF16(jdk.ToLower(jdk.Trim(theWord))))
			}
		}
	}
}

func (d *Dictionary) loadQuantifierDict() {
	d.quantifierDict = newDictSegment(0)
	d.loadDictFile(d.quantifierDict, d.configuration.Path(d.getDictRoot(), pathDicQuantifier), false, "Quantifier")
}

// The surname, suffix and preposition dictionaries are read into tries that are
// thrown away immediately. Nothing queries them — but their absence is fatal,
// so the read itself is the behaviour that matters.
func (d *Dictionary) loadSurnameDict() {
	surnameDict := newDictSegment(0)
	d.loadDictFile(surnameDict, d.configuration.Path(d.getDictRoot(), pathDicSurname), true, "Surname")
}

func (d *Dictionary) loadSuffixDict() {
	suffixDict := newDictSegment(0)
	d.loadDictFile(suffixDict, d.configuration.Path(d.getDictRoot(), pathDicSuffix), true, "Suffix")
}

func (d *Dictionary) loadPrepDict() {
	prepDict := newDictSegment(0)
	d.loadDictFile(prepDict, d.configuration.Path(d.getDictRoot(), pathDicPrep), true, "Preposition")
}

// reLoadMainDict rebuilds the main and stop-word tries in a throw-away
// dictionary and swaps them in, so a reload never leaves a partial trie visible
// to a concurrent segmentation.
func (d *Dictionary) reLoadMainDict() {
	logger.Info("start to reload ik dict.")
	tmpDict := newDictionary(d.configuration)
	tmpDict.configuration = GetSingleton().configuration
	tmpDict.loadMainDict()
	tmpDict.loadStopWordDict()

	d.mu.Lock()
	d.mainDict = tmpDict.mainDict
	d.stopWords = tmpDict.stopWords
	d.mu.Unlock()
	logger.Info("reload ik dict finished.")
}

// remoteWordsClient carries the timeouts the original set on its HTTP GET: ten
// seconds to obtain a connection, ten to establish it, sixty on the socket.
//
// Compression is disabled at the transport so that Go does not negotiate gzip
// behind our back and rewrite the response fields the loader reads. The request
// still advertises gzip, because the original's client does — see readEntity.
var remoteWordsClient = &http.Client{
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		ResponseHeaderTimeout: 60 * time.Second,
		DisableCompression:    true,
	},
}

// acceptEncoding is what the original's HTTP client advertises on every request.
const acceptEncoding = "gzip,deflate"

func getRemoteWords(location string, configuration cfg.Configuration) []string {
	cfg.Check(configuration)
	return getRemoteWordsUnprivileged(location)
}

func getRemoteWordsUnprivileged(location string) []string {
	buffer := make([]string, 0)

	request, err := http.NewRequest(http.MethodGet, location, nil)
	if err != nil {
		logger.Error("getRemoteWords {} error", err, location)
		return buffer
	}
	request.Header.Set("Accept-Encoding", acceptEncoding)

	response, err := remoteWordsClient.Do(request)
	if err != nil {
		logger.Error("getRemoteWords {} error", err, location)
		return buffer
	}
	defer func() { _, _ = io.Copy(io.Discard, response.Body); _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return buffer
	}

	charset := "UTF-8"
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		if at := strings.LastIndex(contentType, "="); strings.Contains(contentType, "charset=") && at >= 0 {
			charset = contentType[at+1:]
		}
	}

	body, contentLength, chunked, err := readEntity(response)
	if err != nil {
		logger.Error("getRemoteWords {} error", err, location)
		return buffer
	}
	// A compressed body reports an unknown length, and a length that is neither
	// positive nor chunked means the loader reads nothing at all. That is what
	// the original does with a gzipped dictionary, so it is what happens here.
	if contentLength <= 0 && !chunked {
		return buffer
	}

	decoded, err := decodeCharset(body, charset)
	if err != nil {
		logger.Error("getRemoteWords {} error", err, location)
		return buffer
	}
	scanner := bufio.NewScanner(strings.NewReader(decoded))
	scanner.Buffer(make([]byte, 0, 512), 1<<20)
	scanner.Split(scanJavaLines)
	for scanner.Scan() {
		buffer = append(buffer, scanner.Text())
	}
	return buffer
}

// readEntity unwraps a compressed response the way the original's HTTP client
// does, and reports the length and chunkedness the loader then tests.
//
// Decompression replaces the entity with a wrapper that reports a content
// length of -1 and inherits the original's chunked flag, and the client drops
// the Content-Length header along with it. A gzipped, non-chunked dictionary
// therefore looks like a body of unknown length and is skipped — faithfully
// reproduced here rather than quietly repaired, because repairing it would load
// words the original never loaded.
func readEntity(response *http.Response) (io.Reader, int64, bool, error) {
	chunked := false
	for _, encoding := range response.TransferEncoding {
		if encoding == "chunked" {
			chunked = true
		}
	}

	contentEncoding := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Encoding"), ",")[0]))
	if response.ContentLength == 0 {
		// An empty entity is never unwrapped.
		return response.Body, response.ContentLength, chunked, nil
	}

	switch contentEncoding {
	case "gzip", "x-gzip":
		reader, err := gzip.NewReader(response.Body)
		if err != nil {
			return nil, 0, false, err
		}
		return reader, -1, chunked, nil
	case "deflate":
		// Both the zlib-wrapped and the raw form are accepted, as they are by
		// the original's deflate entity.
		data, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, 0, false, err
		}
		if zlibReader, err := zlib.NewReader(bytes.NewReader(data)); err == nil {
			return zlibReader, -1, chunked, nil
		}
		return flate.NewReader(bytes.NewReader(data)), -1, chunked, nil
	default:
		return response.Body, response.ContentLength, chunked, nil
	}
}

// decodeCharset reads the body under the charset the server named. Only the
// charsets a Go program can decode without a third-party table are supported;
// anything else is reported the way Java reported an unsupported encoding.
func decodeCharset(body io.Reader, charset string) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(charset) {
	case "utf-8", "utf8", "us-ascii", "ascii":
		return string(data), nil
	case "iso-8859-1", "latin1", "iso8859-1":
		runes := make([]rune, len(data))
		for i, b := range data {
			runes[i] = rune(b)
		}
		return string(runes), nil
	default:
		return "", fmt.Errorf("unsupported charset %q", charset)
	}
}
