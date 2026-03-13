package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PushTokenRecord struct {
	Token     string    `json:"token"`
	Platform  string    `json:"platform"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PushTokenStore struct {
	path   string
	mu     sync.Mutex
	loaded bool
	cache  map[string][]PushTokenRecord
}

func NewPushTokenStore(path string) *PushTokenStore {
	return &PushTokenStore{path: path}
}

func (s *PushTokenStore) Put(key, token, platform string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.loadAllLocked()
	if err != nil {
		return err
	}
	records := all[key]
	trimmedToken := strings.TrimSpace(token)
	filtered := make([]PushTokenRecord, 0, len(records)+1)
	for _, item := range records {
		if strings.TrimSpace(item.Token) == trimmedToken {
			continue
		}
		filtered = append(filtered, item)
	}
	filtered = append(filtered, PushTokenRecord{
		Token:     trimmedToken,
		Platform:  strings.TrimSpace(platform),
		UpdatedAt: time.Now().UTC(),
	})
	all[key] = filtered
	return s.writeAllLocked(all)
}

func (s *PushTokenStore) Delete(key, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.loadAllLocked()
	if err != nil {
		return err
	}
	records := all[key]
	filtered := make([]PushTokenRecord, 0, len(records))
	for _, item := range records {
		if strings.TrimSpace(item.Token) == strings.TrimSpace(token) {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		delete(all, key)
	} else {
		all[key] = filtered
	}
	return s.writeAllLocked(all)
}

func (s *PushTokenStore) MoveTokenToKey(targetKey, token, platform string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.loadAllLocked()
	if err != nil {
		return err
	}
	trimmedToken := strings.TrimSpace(token)
	for key, records := range all {
		filtered := make([]PushTokenRecord, 0, len(records))
		for _, item := range records {
			if strings.TrimSpace(item.Token) == trimmedToken {
				continue
			}
			filtered = append(filtered, item)
		}
		if len(filtered) == 0 {
			delete(all, key)
		} else {
			all[key] = filtered
		}
	}
	all[strings.TrimSpace(targetKey)] = []PushTokenRecord{{
		Token:     trimmedToken,
		Platform:  strings.TrimSpace(platform),
		UpdatedAt: time.Now().UTC(),
	}}
	return s.writeAllLocked(all)
}

func (s *PushTokenStore) List(key string) ([]PushTokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.loadAllLocked()
	if err != nil {
		return nil, err
	}
	return append([]PushTokenRecord(nil), all[key]...), nil
}

func (s *PushTokenStore) loadAllLocked() (map[string][]PushTokenRecord, error) {
	if s.loaded {
		if s.cache == nil {
			s.cache = map[string][]PushTokenRecord{}
		}
		return s.cache, nil
	}
	all, err := s.readAllLocked()
	if err != nil {
		return nil, err
	}
	s.cache = all
	s.loaded = true
	return s.cache, nil
}

func (s *PushTokenStore) readAllLocked() (map[string][]PushTokenRecord, error) {
	if _, err := os.Stat(s.path); err != nil {
		if os.IsNotExist(err) {
			return map[string][]PushTokenRecord{}, nil
		}
		return nil, err
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string][]PushTokenRecord{}, nil
	}

	var data map[string][]PushTokenRecord
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string][]PushTokenRecord{}
	}
	return data, nil
}

func (s *PushTokenStore) writeAllLocked(data map[string][]PushTokenRecord) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "push-tokens-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return err
	}
	s.cache = clonePushTokenRecordMap(data)
	s.loaded = true
	return nil
}

func clonePushTokenRecordMap(input map[string][]PushTokenRecord) map[string][]PushTokenRecord {
	cloned := make(map[string][]PushTokenRecord, len(input))
	for key, value := range input {
		cloned[key] = append([]PushTokenRecord(nil), value...)
	}
	return cloned
}
