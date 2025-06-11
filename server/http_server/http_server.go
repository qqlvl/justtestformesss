package http_server

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"time"
	"website_proxier/encoding"
	"website_proxier/proxy_pool"
	"website_proxier/siteconfig"
)

var cacheRelatedHeaders = []string{
	"Age",
	"Date",
	"Cache-Control",
	"Date",
	"ETag",
	"Expires",
	"Last-Modified",
	"Pragma",
	"Strict-Transport-Security",
}

var noStripHeadersFrom = []string{
	"png",
	"jpg",
	"svg",
	"css",
	"ico",
}

var stripHeaders = []string{
	"cdn-loop", "cf-connecting-ip", "cf-ipcountry", "cf-ray", "cf-visitor", "x-forwarded-for", "x-forwarded-proto", "x-real-ip",
}

const httpClientTtl = time.Minute * 5

type httpClientWithTtl struct {
	Client     *http.Client
	LastUsedAt time.Time
}

var httpClients []*httpClientWithTtl
var httpClientIndex = atomic.Uint64{}

func getHttpClient() *httpClientWithTtl {
	if len(httpClients) == 1 {
		return httpClients[0]
	}
	index := httpClientIndex.Add(1)
	return httpClients[index%uint64(len(httpClients))]
}
func isRemoteAddrLocal(remoteAddr string) bool {
	if strings.HasPrefix(remoteAddr, "127.") {
		return true
	}
	if strings.HasPrefix(remoteAddr, "192.168.") {
		return true
	}
	if strings.HasPrefix(remoteAddr, "10.") {
		return true
	}
	if strings.HasPrefix(remoteAddr, "localhost") {
		return true
	}

	return false
}

func HandleRequest(w http.ResponseWriter, r *http.Request) {
	logrus.WithFields(logrus.Fields{
		"method": r.Method,
		"host":   r.Host,
		"path":   r.URL.Path,
	}).Info("Received request")
	host := r.Host
	if host == "" {
		logrus.Warn("Host is empty")
		http.Error(w, "Host is empty", http.StatusBadRequest)
		return
	}
	path := r.URL.Path

	if path == "/reload_all_configs" {
		_ = siteconfig.LoadAllSites()
		logrus.Info("All configs reloaded")
		w.WriteHeader(http.StatusOK)
		return
	}

	if strings.HasPrefix(path, "/reload_specific_config") {
		configName := r.URL.Query().Get("config_name")
		if configName == "" {
			logrus.Warn("Host is empty")
			http.Error(w, "Host is empty", http.StatusBadRequest)
			return
		}
		config, ok := siteconfig.GetBaseConfigByName(configName)
		if !ok {
			logrus.WithField("config_name", configName).Warn("Config not found")
			http.Error(w, "Config not found", http.StatusNotFound)
		}

		err := config.Load()
		if err != nil {
			logrus.WithError(err).Error("Error reloading config")
			http.Error(w, "Error reloading config", http.StatusInternalServerError)
			return
		}
		logrus.WithField("config_name", configName).Info("Config reloaded")
		w.WriteHeader(http.StatusOK)
		return
	}

	site, ok := siteconfig.GetSiteConfig(host)
	if !ok || site.BaseConfig.Deactivated {
		logrus.WithFields(logrus.Fields{
			"host": host,
		}).Warn("Website not found")
		w.Header().Set("Location", "https://www.godaddy.com/websites/website-builder")
		w.WriteHeader(http.StatusFound)
		return
	}

	remoteAddr := r.RemoteAddr

	if xForwardedFor := r.Header.Get("X-Real-IP"); xForwardedFor != "" && isRemoteAddrLocal(remoteAddr) {
		remoteAddr = xForwardedFor
	}

	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	if site.ShouldBlock(path) {
		logrus.WithFields(site.LogrusFields()).WithField("path", path).WithField("remote_addr", remoteAddr).Warn("Blocked")
		http.Error(w, "", http.StatusForbidden)
	}
	logr := logrus.WithFields(site.LogrusFields()).WithField("path", path).WithField("remote_addr", remoteAddr).WithField("host", host)
	//logr.Info("Handling request")
	startedAt := time.Now()

	if path == "/reload_config" {
		_ = site.BaseConfig.Load()
		logr.Info("Config reloaded")
		w.WriteHeader(http.StatusOK)
		return
	}

	defer func() {
		logr.WithField("duration", time.Since(startedAt).Round(time.Millisecond)).Info("Request finished")
	}()

	if r.Header.Get("Content-Encoding") == "identity" {
		r.Header.Del("Content-Encoding")
	}

	if entry, ok := site.ProbeCache(path); ok {
		logr.Info("Returning from cache")
		for key, value := range entry.Headers {
			w.Header().Set(key, value)
		}
		for key, value := range site.RespHeadersOverride {
			w.Header().Set(key, value)
		}
		w.Header().Del("Content-Length")
		body, retEncoding, err := encoding.EncodeWithSomething(site.Replace(entry.Content), r.Header.Get("Accept-Encoding"))
		if err != nil {
			logr.WithError(err).Error("Error encoding body")
			http.Error(w, "Error encoding body", http.StatusInternalServerError)
			return
		}
		if retEncoding != "" {
			w.Header().Set("Content-Encoding", retEncoding)
		}
		wasReplaced := len(body) != len(entry.Content) || bytes.Compare(body, entry.Content) != 0
		if wasReplaced {
			for _, header := range cacheRelatedHeaders {
				delete(w.Header(), header)
			}
			w.Header().Set("X-Replaced", "1")
			w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Pragma", "no-cache")
		}
		w.WriteHeader(entry.Status)
		_, _ = w.Write(body)
		return
	}

	client := getHttpClient()

	if client == nil {
		logr.Error("Error getting client")
		http.Error(w, "Error getting client", http.StatusInternalServerError)
		return
	}

	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		logr.WithError(err).Error("Error reading body")
		http.Error(w, "Error reading body", http.StatusInternalServerError)
		return
	}

	retryNum := -1

retry:
	retryNum++
	if retryNum > 0 {
		client = getHttpClient()
	}
	if retryNum > 3 {
		logr.Error("Too many retries")
		http.Error(w, "Failed to load", http.StatusInternalServerError)
		return
	}
	req, err := http.NewRequest(r.Method, site.URL(path), bytes.NewReader(reqBody))
	if err != nil {
		logr.WithError(err).Error("Error creating request")
		http.Error(w, "Error creating request", http.StatusInternalServerError)
		return
	}

	for key, value := range r.Header {
		if key == "Origin" || key == "Referer" || slices.Contains(stripHeaders, strings.ToLower(key)) {
			continue
		}
		req.Header.Set(key, value[0])
	}

	req.Header.Set("Origin", strings.Replace(r.Header.Get("Origin"), r.Host, site.TargetHost, 1))
	req.Header.Set("Referer", strings.Replace(r.Header.Get("Referer"), r.Host, site.TargetHost, 1))

	if referrer := r.Header.Get("Referer"); referrer != "" {
		referrer = strings.Replace(referrer, r.Host, site.TargetHost, 1)
		req.Header.Set("Referer", referrer)
	}

	req.Header.Set("Host", site.TargetHost)
	req.Header.Set("Connection", "keep-alive")

	for k, v := range site.ReqHeadersOverride {
		req.Header.Set(k, v)
	}

	logr.Infof("Incoming: [%s] %s %+v", r.Method, r.URL, r.Header)
	logr.Infof("Outgoing: [%s] %s %+v", req.Method, req.URL.String(), req.Header)

	resp, err := client.Client.Do(req)
	if err != nil {
		logr.WithError(err).Error("Error getting page")
		http.Error(w, "Error getting page", http.StatusInternalServerError)
		return
	}
	if resp.StatusCode == 503 {
		_ = resp.Body.Close()
		logr.Warn("503, retrying")
		goto retry
	}
	defer resp.Body.Close()
	originalBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logr.WithError(err).Error("Error reading body")
		http.Error(w, "Error reading body", http.StatusInternalServerError)
		return
	}

	originalBody, err = encoding.Decode(originalBody, resp.Header.Get("Content-Encoding"))
	if err != nil {
		logr.WithError(err).Error("Error decoding body")
		http.Error(w, "Error decoding body", http.StatusInternalServerError)
		return
	}
	var newBody []byte
	newBody = site.Replace(originalBody)

	wasReplaced := len(newBody) != len(originalBody) || bytes.Compare(newBody, originalBody) != 0

	newBody, err = encoding.Encode(newBody, resp.Header.Get("Content-Encoding"))
	if err != nil {
		logr.WithError(err).Error("Error encoding body")
		http.Error(w, "Error encoding body", http.StatusInternalServerError)
		return
	}

	var headers = make(map[string]string)
	for key, value := range resp.Header {
		headers[key] = value[0]
	}

	shouldStripHeaders := true
	for _, ext := range noStripHeadersFrom {
		if strings.HasSuffix(path, ext) {
			shouldStripHeaders = false
			break
		}
	}
	if shouldStripHeaders {
		for _, header := range stripHeaders {
			delete(headers, header)
		}
	}

	for k, v := range site.RespHeadersOverride {
		headers[k] = v
	}

	delete(headers, "Content-Length")

	if resp.StatusCode > 399 {
		logr.Warnf("Response: %d %+v", resp.StatusCode, headers)
	}

	if resp.StatusCode < 299 {
		site.MbSaveToCache(path, originalBody, headers, resp.StatusCode)
	}
	for key, value := range headers {
		w.Header().Set(key, value)
	}
	if wasReplaced {
		for _, header := range cacheRelatedHeaders {
			delete(w.Header(), header)
		}
	}
	if wasReplaced || resp.StatusCode > 499 {
		w.Header().Set("X-Replaced", "1")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Pragma", "no-cache")
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(newBody)
	return
}

func StartServer() {
	httpClients = make([]*httpClientWithTtl, len(proxy_pool.GetAllProxies()))
	for i, proxy := range proxy_pool.GetAllProxies() {
		transport := &http.Transport{
			MaxIdleConns:          20,
			DisableCompression:    false,
			IdleConnTimeout:       time.Second * 30,
			TLSHandshakeTimeout:   time.Second * 5,
			ResponseHeaderTimeout: time.Second * 40,
		}

		proxyUrl, err := url.Parse(proxy)
		if err != nil {
			logrus.WithError(err).Error("Error parsing proxy")
		}
		transport.Proxy = http.ProxyURL(proxyUrl)

		httpClients[i] = &httpClientWithTtl{
			Client: &http.Client{
				Transport: transport,
				Timeout:   time.Second * 40,
			},
			LastUsedAt: time.Now(),
		}
	}

	// check that there are no nil pointers
	for _, client := range httpClients {
		if client == nil {
			logrus.Error("httpClients contains nil pointer")
			return
		}
		if client.Client == nil {
			logrus.Error("httpClients contains nil pointer")
			return
		}
		if client.Client.Transport == nil {
			logrus.Error("httpClients contains nil pointer")
			return
		}
	}

	if len(httpClients) == 0 {
		httpClients = append(httpClients, &httpClientWithTtl{
			Client: &http.Client{
				Timeout: time.Second * 40,
				Transport: &http.Transport{
					MaxIdleConns:          20,
					DisableCompression:    false,
					IdleConnTimeout:       time.Second * 30,
					TLSHandshakeTimeout:   time.Second * 5,
					ResponseHeaderTimeout: time.Second * 5,
				},
			},
		})
	}

	http.HandleFunc("/", HandleRequest)
	logrus.Info("Starting server")
	err := http.ListenAndServe(":6688", nil)
	if err != nil {
		logrus.WithError(err).Fatal("Error starting server")
	}
}
