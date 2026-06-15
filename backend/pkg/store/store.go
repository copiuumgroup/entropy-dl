package store

import (
        "encoding/json"
        "os"
        "path/filepath"

        "go.etcd.io/bbolt"
)

type Settings struct {
        OutputDir      string `json:"output_dir"`
        MaxWorkers     int    `json:"max_workers"`
        BandwidthLimit string `json:"bandwidth_limit"` // global default bandwidth limit (e.g., "5M", "10M", "0" = unlimited)
        OnboardingDone bool   `json:"onboarding_done"`
}

type Store struct {
        db *bbolt.DB
}

var (
        bucketJobs     = []byte("jobs")
        bucketSettings = []byte("settings")
        keySettings    = []byte("global_settings")
)

func New(path string) (*Store, error) {
        if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
                return nil, err
        }
        db, err := bbolt.Open(path, 0o600, nil)
        if err != nil {
                return nil, err
        }
        err = db.Update(func(tx *bbolt.Tx) error {
                _, err := tx.CreateBucketIfNotExists(bucketJobs)
                if err != nil {
                        return err
                }
                _, err = tx.CreateBucketIfNotExists(bucketSettings)
                return err
        })
        if err != nil {
                return nil, err
        }
        return &Store{db: db}, nil
}

func (s *Store) Close() error {
        return s.db.Close()
}

func (s *Store) LoadSettings() (Settings, error) {
        var settings Settings
        err := s.db.View(func(tx *bbolt.Tx) error {
                b := tx.Bucket(bucketSettings)
                v := b.Get(keySettings)
                if len(v) > 0 {
                        return json.Unmarshal(v, &settings)
                }
                return nil
        })
        return settings, err
}

func (s *Store) SaveSettings(settings Settings) error {
        return s.db.Update(func(tx *bbolt.Tx) error {
                b := tx.Bucket(bucketSettings)
                data, err := json.Marshal(settings)
                if err != nil {
                        return err
                }
                return b.Put(keySettings, data)
        })
}

// LoadJobs iterates over the jobs bucket and returns all raw JSON jobs.
func (s *Store) LoadJobs() ([][]byte, error) {
        var jobs [][]byte
        err := s.db.View(func(tx *bbolt.Tx) error {
                b := tx.Bucket(bucketJobs)
                return b.ForEach(func(k, v []byte) error {
                        cp := make([]byte, len(v))
                        copy(cp, v)
                        jobs = append(jobs, cp)
                        return nil
                })
        })
        return jobs, err
}

// SaveJob writes a single job to the jobs bucket.
func (s *Store) SaveJob(id string, data []byte) error {
        return s.db.Update(func(tx *bbolt.Tx) error {
                b := tx.Bucket(bucketJobs)
                return b.Put([]byte(id), data)
        })
}

// DeleteJob removes a job from the jobs bucket.
func (s *Store) DeleteJob(id string) error {
        return s.db.Update(func(tx *bbolt.Tx) error {
                b := tx.Bucket(bucketJobs)
                return b.Delete([]byte(id))
        })
}
