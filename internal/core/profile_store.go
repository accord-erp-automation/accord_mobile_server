package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type ProfilePrefs struct {
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type ProfileStore struct {
	path string
	mu   sync.Mutex
}

func NewProfileStore(path string) *ProfileStore {
	return &ProfileStore{path: path}
}

func (s *ProfileStore) Get(key string) (ProfilePrefs, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.readAllLocked()
	if err != nil {
		return ProfilePrefs{}, err
	}
	return all[key], nil
}

func (s *ProfileStore) Put(key string, prefs ProfilePrefs) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.readAllLocked()
	if err != nil {
		return err
	}
	all[key] = prefs
	return s.writeAllLocked(all)
}

func (s *ProfileStore) readAllLocked() (map[string]ProfilePrefs, error) {
	if _, err := os.Stat(s.path); err != nil {
		if os.IsNotExist(err) {
			return map[string]ProfilePrefs{}, nil
		}
		return nil, err
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]ProfilePrefs{}, nil
	}

	var data map[string]ProfilePrefs
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]ProfilePrefs{}
	}
	return data, nil
}

func (s *ProfileStore) writeAllLocked(data map[string]ProfilePrefs) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), "profile-prefs-*.json")
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
	return os.Rename(tmpPath, s.path)
}
