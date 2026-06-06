package staging

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Baseline struct {
	Missing bool   `json:"missing,omitempty"`
	Blob    string `json:"blob,omitempty"`
}

type Op struct {
	CpSeq    int    `json:"cp_seq"`
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	RenameTo string `json:"rename_to,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

type Store struct {
	Dir       string               `json:"-"`
	Baselines map[string]Baseline  `json:"baselines"`
	Ops       []Op                 `json:"ops"`
}

func Load(dir string) (*Store, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, os.ErrInvalid
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	meta := filepath.Join(dir, "staging.json")
	b, err := os.ReadFile(meta)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{Dir: dir, Baselines: make(map[string]Baseline)}, nil
		}
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	s.Dir = dir
	if s.Baselines == nil {
		s.Baselines = make(map[string]Baseline)
	}
	return &s, nil
}

func (s *Store) Save() error {
	if s == nil {
		return nil
	}
	meta := filepath.Join(s.Dir, "staging.json")
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := meta + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, meta)
}

func (s *Store) RecordBefore(absPath string) error {
	if s == nil {
		return nil
	}
	absPath = filepath.Clean(absPath)
	if _, ok := s.Baselines[absPath]; ok {
		return nil
	}
	b, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.Baselines[absPath] = Baseline{Missing: true}
			return s.Save()
		}
		return err
	}
	blob, err := s.writeBlob(b)
	if err != nil {
		return err
	}
	s.Baselines[absPath] = Baseline{Blob: blob}
	return s.Save()
}

func (s *Store) RecordOp(cpSeq int, kind, absPath, renameTo string, content []byte) error {
	if s == nil {
		return nil
	}
	absPath = filepath.Clean(absPath)
	renameTo = filepath.Clean(renameTo)
	op := Op{CpSeq: cpSeq, Kind: kind, Path: absPath, RenameTo: renameTo}
	if content != nil {
		blob, err := s.writeBlob(content)
		if err != nil {
			return err
		}
		op.Blob = blob
	}
	s.Ops = append(s.Ops, op)
	return s.Save()
}

func (s *Store) writeBlob(data []byte) (string, error) {
	name := filepath.Join(s.Dir, "blobs", randomBlobName())
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(name, data, 0o600); err != nil {
		return "", err
	}
	rel, err := filepath.Rel(s.Dir, name)
	if err != nil {
		return name, nil
	}
	return rel, nil
}

func (s *Store) readBlob(rel string) ([]byte, error) {
	if rel == "" {
		return nil, nil
	}
	return os.ReadFile(filepath.Join(s.Dir, rel))
}

func randomBlobName() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return filepath.Join("blobs", hex.EncodeToString(b[:])+".bin")
}

func sortedOps(ops []Op) []Op {
	out := append([]Op(nil), ops...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CpSeq != out[j].CpSeq {
			return out[i].CpSeq < out[j].CpSeq
		}
		return i < j
	})
	return out
}
