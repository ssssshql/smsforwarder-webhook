package store

import (
	"sync"
	"time"
)

// Entry 验证码条目
type Entry struct {
	Code      string    `json:"code"`
	Type      string    `json:"type,omitempty"`
	Phone     string    `json:"phone,omitempty"` // SIM 标识，如 SIM1_152977314482
	From      string    `json:"from,omitempty"`
	Content   string    `json:"content,omitempty"`
	Timestamp int64     `json:"timestamp,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpireAt  time.Time `json:"expire_at"`
}

// key 按 type + phone 分槽，每个组合仅保留最近一条
func key(msgType, phone string) string {
	return msgType + "\x00" + phone
}

// Store 内存仓库：每个 (type, phone) 仅保留最近一条，进程重启后清空
type Store struct {
	mu      sync.RWMutex
	entries map[string]Entry
	ttl     time.Duration
}

func New(ttl time.Duration) *Store {
	return &Store{
		entries: make(map[string]Entry),
		ttl:     ttl,
	}
}

func (s *Store) Put(from, msgType, phone, content, code string, ts int64) Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	e := Entry{
		Code:      code,
		Type:      msgType,
		Phone:     phone,
		From:      from,
		Content:   content,
		Timestamp: ts,
		CreatedAt: now,
		ExpireAt:  now.Add(s.ttl),
	}
	s.entries[key(msgType, phone)] = e
	return e
}

// Get 按 type + phone 读取；任一为空则忽略该条件
func (s *Store) Get(msgType, phone string, consume bool) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	// 精确匹配
	if msgType != "" || phone != "" {
		if msgType != "" && phone != "" {
			return s.take(key(msgType, phone), consume, now)
		}
		// 仅 type 或仅 phone：找匹配且最新的一条
		var best Entry
		var bestKey string
		found := false
		for k, e := range s.entries {
			if now.After(e.ExpireAt) {
				delete(s.entries, k)
				continue
			}
			if msgType != "" && e.Type != msgType {
				continue
			}
			if phone != "" && e.Phone != phone {
				continue
			}
			if !found || e.CreatedAt.After(best.CreatedAt) {
				best = e
				bestKey = k
				found = true
			}
		}
		if !found {
			return Entry{}, false
		}
		if consume {
			delete(s.entries, bestKey)
		}
		return best, true
	}

	// 未指定条件：取全局最新一条
	var best Entry
	var bestKey string
	found := false
	for k, e := range s.entries {
		if now.After(e.ExpireAt) {
			delete(s.entries, k)
			continue
		}
		if !found || e.CreatedAt.After(best.CreatedAt) {
			best = e
			bestKey = k
			found = true
		}
	}
	if !found {
		return Entry{}, false
	}
	if consume {
		delete(s.entries, bestKey)
	}
	return best, true
}

func (s *Store) take(k string, consume bool, now time.Time) (Entry, bool) {
	e, ok := s.entries[k]
	if !ok {
		return Entry{}, false
	}
	if now.After(e.ExpireAt) {
		delete(s.entries, k)
		return Entry{}, false
	}
	if consume {
		delete(s.entries, k)
	}
	return e, true
}

func (s *Store) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, e := range s.entries {
		if now.After(e.ExpireAt) {
			delete(s.entries, k)
		}
	}
}
