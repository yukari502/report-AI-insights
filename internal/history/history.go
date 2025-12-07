package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type History struct {
	mu         sync.RWMutex
	path       string
	Crawled    map[string]string `json:"crawled_urls"`    // URL -> Date
	Summarized map[string]string `json:"summarized_urls"` // URL -> Date
}

func NewHistory(path string) (*History, error) {
	h := &History{
		path:       path,
		Crawled:    make(map[string]string),
		Summarized: make(map[string]string),
	}
	if err := h.load(); err != nil {
		// If load fails (e.g. file doesn't exist), we start empty but try to ensure dir exists
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return nil, err
			}
			return h, nil
		}
		return nil, err
	}
	return h, nil
}

func (h *History) load() error {
	data, err := os.ReadFile(h.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, h)
}

func (h *History) Save() error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.save()
}

func (h *History) save() error {
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.path, data, 0644)
}

func (h *History) HasCrawled(url string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.Crawled[url]
	return exists
}

func (h *History) AddCrawled(url string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Crawled[url] = time.Now().Format("2006-01-02")
	_ = h.save() // Ignore error for now
}

func (h *History) HasSummarized(url string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.Summarized[url]
	return exists
}

func (h *History) AddSummarized(url string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Summarized[url] = time.Now().Format("2006-01-02")
	_ = h.save()
}
