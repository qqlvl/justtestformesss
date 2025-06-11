package siteconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"maps"
	"os"
	"slices"
	"sync"
	"time"
)

const configDir = "configs_v2"

var (
	websites    = make(map[string]*WebsiteConfig)
	websiteLock sync.RWMutex
	baseConfigs = make(map[string]*SiteBaseConfig)
	baseLock    sync.RWMutex
)

type SiteBaseConfig struct {
	Name string

	Vars           map[string]string         `json:"vars"`
	Websites       map[string]string         `json:"websites"`        // original website domain -> new website domain
	WebsiteConfigs map[string]*WebsiteConfig `json:"website_configs"` // new website domain -> website config
	Deactivated    bool                      `json:"deactivated"`
}

func (s *SiteBaseConfig) Load() error {
	baseConfigFile, err := os.Open(configDir + "/" + s.Name + "/config.json")
	if err != nil {
		return err
	}

	temp := &SiteBaseConfig{}
	err = json.NewDecoder(baseConfigFile).Decode(temp)
	_ = baseConfigFile.Close()
	if err != nil {
		return err
	}

	previousWebsiteConfigs := make(map[string]*WebsiteConfig, len(s.WebsiteConfigs))

	maps.Copy(previousWebsiteConfigs, s.WebsiteConfigs)

	s.Vars = temp.Vars
	s.Websites = temp.Websites
	s.WebsiteConfigs = make(map[string]*WebsiteConfig, len(s.Websites))

	// apply vars to websites, as they most likely contain vars
	for k := range s.Websites {
		s.Websites[k], err = formatString(s.Websites[k], s.Vars)
		if err != nil {
			return fmt.Errorf("error formatting website domain %s: %w", k, err)
		}
	}

	for k, v := range s.Websites {
		configFile, err := os.Open(configDir + "/" + s.Name + "/" + k + ".json")
		if err != nil {
			return fmt.Errorf("error opening website config file %s: %w", k, err)
		}

		websiteConfig := &WebsiteConfig{
			TargetHost: k,
			BaseConfig: s,
		}
		err = json.NewDecoder(configFile).Decode(websiteConfig)
		_ = configFile.Close()
		if err != nil {
			return fmt.Errorf("error decoding website config file %s: %w", k, err)
		}
		websiteConfig.init()

		// apply vars to replacements
		for i := range websiteConfig.Replacements {
			websiteConfig.Replacements[i].From, err = formatString(websiteConfig.Replacements[i].From, s.Vars)
			if err != nil {
				return fmt.Errorf("error formatting replacement From %s: %w", websiteConfig.Replacements[i].From, err)
			}
			websiteConfig.Replacements[i].To, err = formatString(websiteConfig.Replacements[i].To, s.Vars)
			if err != nil {
				return fmt.Errorf("error formatting replacement To %s: %w", websiteConfig.Replacements[i].To, err)
			}
		}

		if previousWebsiteConfig, ok := previousWebsiteConfigs[v]; ok {
			maps.Copy(websiteConfig.cache, previousWebsiteConfig.cache)
		}

		s.WebsiteConfigs[v] = websiteConfig
	}

	websiteLock.Lock()

	// remove all previous website configs
	for k := range previousWebsiteConfigs {
		delete(websites, k)
	}

	// add new website configs
	for _, v := range s.Websites {
		websites[v] = s.WebsiteConfigs[v]
	}
	websiteLock.Unlock()

	return nil
}

func (s *SiteBaseConfig) Cleanup() {
	websiteLock.Lock()
	for _, v := range s.Websites {
		delete(websites, v)
	}
	websiteLock.Unlock()

	baseLock.Lock()
	delete(baseConfigs, s.Name)
	baseLock.Unlock()
}

func (s *SiteBaseConfig) LogrusFields() logrus.Fields {
	return logrus.Fields{
		"name": s.Name,
	}
}

type WebsiteConfig struct {
	TargetHost string // set from SiteBaseConfig.Websites
	BaseConfig *SiteBaseConfig

	LoadedAt time.Time
	cache    map[string]*PageCacheEntry
	cacheMu  sync.Mutex

	NoCache             bool              `json:"no_cache"`
	ReqHeadersOverride  map[string]string `json:"req_headers_override"`
	RespHeadersOverride map[string]string `json:"resp_headers_override"`
	Replacements        []Replacement     `json:"replacements"`
	BypassCacheFor      []string          `json:"bypass_cache_for"`
	Block               []string          `json:"block"`
}

func (w *WebsiteConfig) LogrusFields() logrus.Fields {
	return logrus.Fields{
		"name":        w.BaseConfig.Name,
		"target_host": w.TargetHost,
		"http_host":   w.BaseConfig.Websites[w.TargetHost],
	}
}

func (w *WebsiteConfig) LogrusFieldsWithAction(action string) logrus.Fields {
	fields := w.LogrusFields()
	fields["action"] = action
	return fields
}

func (w *WebsiteConfig) init() {
	w.LoadedAt = time.Now()
	w.cache = make(map[string]*PageCacheEntry)
	if w.ReqHeadersOverride == nil {
		w.ReqHeadersOverride = make(map[string]string)
	}
	if w.RespHeadersOverride == nil {
		w.RespHeadersOverride = make(map[string]string)
	}
	if w.Replacements == nil {
		w.Replacements = make([]Replacement, 0)
	}
	if w.BypassCacheFor == nil {
		w.BypassCacheFor = make([]string, 0)
	}
	if w.Block == nil {
		w.Block = make([]string, 0)
	}
}

func (w *WebsiteConfig) ProbeCache(path string) (*PageCacheEntry, bool) {
	if w.NoCache {
		return nil, false
	}
	w.cacheMu.Lock()
	defer w.cacheMu.Unlock()

	entry, ok := w.cache[path]
	if ok {
		if time.Since(entry.FetchedAt) > cacheTtl {
			ok = false
		}
	}
	if ok {
		logrus.WithFields(w.LogrusFieldsWithAction("probe_cache")).WithField("path", path).Info("Returned from cache")
	}
	return entry, ok
}

func (w *WebsiteConfig) ShouldBlock(path string) bool {
	return slices.Contains(w.Block, path)
}

func (w *WebsiteConfig) MbSaveToCache(path string, content []byte, headers map[string]string, status int) {
	if w.NoCache {
		return
	}
	w.cacheMu.Lock()
	defer w.cacheMu.Unlock()

	if slices.Contains(w.BypassCacheFor, path) {
		return
	}
	var headersToCache = make(map[string]string)
	for _, header := range cacheHeaders {
		if header == "Content-Encoding" || header == "content-encoding" {
			continue
		}
		if value, ok := headers[header]; ok {
			headersToCache[header] = value
		}
	}

	w.cache[path] = &PageCacheEntry{
		Path:      path,
		FetchedAt: time.Now(),
		Content:   content,
		Headers:   headersToCache,
		Status:    status,
	}

	logrus.WithFields(w.LogrusFieldsWithAction("save_to_cache")).WithField("path", path).Info("Saved to cache")
}

func (w *WebsiteConfig) Replace(content []byte) []byte {
	for _, replacement := range w.Replacements {
		content = replacement.Replace(content)
	}
	return content
}

func (w *WebsiteConfig) URL(path string) string {
	return "https://" + w.TargetHost + path
}

func GetSiteConfig(host string) (*WebsiteConfig, bool) {
	websiteLock.RLock()
	defer websiteLock.RUnlock()

	config, ok := websites[host]
	return config, ok
}

func GetBaseConfigByName(name string) (*SiteBaseConfig, bool) {
	baseLock.RLock()
	defer baseLock.RUnlock()

	config, ok := baseConfigs[name]
	return config, ok
}

func loadSite(name string) (*SiteBaseConfig, error) {
	stat, err := os.Stat(configDir + "/" + name)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("site %s is not a directory", name)
	}

	baseConfig := &SiteBaseConfig{
		Name: name,
	}

	err = baseConfig.Load()
	if err != nil {
		return nil, err
	}

	baseLock.Lock()
	baseConfigs[name] = baseConfig
	baseLock.Unlock()

	return baseConfig, nil
}

func LoadAllSites() error {
	for _, baseConfig := range baseConfigs {
		baseConfig.Cleanup()
	}

	dir, err := os.ReadDir(configDir)
	if err != nil {
		return err
	}

	errs := make([]error, 0, len(dir))

	for _, entry := range dir {
		if !entry.IsDir() {
			continue
		}

		_, err := loadSite(entry.Name())
		if err != nil {
			errs = append(errs, fmt.Errorf("error loading site %s: %w", entry.Name(), err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	for _, baseConfig := range baseConfigs {
		logrus.WithFields(baseConfig.LogrusFields()).Info("Loaded site")
	}

	for _, website := range websites {
		logrus.WithFields(website.LogrusFields()).Info("Loaded website")
	}

	return nil
}

func init() {
	err := LoadAllSites()
	if err != nil {
		panic(err)
	}
}
