package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/oliveagle/zvec-go"
)

var errCollectionExists = errors.New("collection already exists")
var errCollectionNotFound = errors.New("collection not found")

// collectionNameRe restricts collection names to a filesystem/URL-safe charset.
var collectionNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// ValidateCollectionName reports whether name is a valid collection name.
func ValidateCollectionName(name string) bool {
	return collectionNameRe.MatchString(name)
}

// CollectionManager holds a set of open zvec collections under a data
// directory, keyed by name. It is the multi-collection facade the HTTP service
// exposes ("allow creating multiple collections or tables").
type CollectionManager struct {
	mu      sync.RWMutex
	dataDir string
	colls   map[string]*zvec.Collection
}

// NewCollectionManager creates a manager rooted at dataDir and ensures the
// directory exists.
func NewCollectionManager(dataDir string) (*CollectionManager, error) {
	if dataDir == "" {
		dataDir = "./data"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir %q: %w", dataDir, err)
	}
	return &CollectionManager{
		dataDir: dataDir,
		colls:   make(map[string]*zvec.Collection),
	}, nil
}

// OpenExisting opens every collection that already exists under the data
// directory. It is called at startup so restarts preserve collections.
func (m *CollectionManager) OpenExisting() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := os.ReadDir(m.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if _, loaded := m.colls[name]; loaded {
			continue
		}
		if _, err := os.Stat(filepath.Join(m.dataDir, name, "collection.json")); err != nil {
			continue // not a collection
		}
		coll, err := zvec.Open(filepath.Join(m.dataDir, name), nil)
		if err != nil {
			continue // skip unreadable collections rather than fail startup
		}
		m.colls[name] = coll
	}
	return nil
}

// Create creates and opens a new collection with the given schema.
func (m *CollectionManager) Create(name string, schema *zvec.CollectionSchema) (*zvec.Collection, error) {
	if !ValidateCollectionName(name) {
		return nil, fmt.Errorf("invalid collection name %q", name)
	}
	if schema == nil {
		return nil, errors.New("schema is required")
	}
	if err := schema.Validate(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.colls[name]; exists {
		return nil, errCollectionExists
	}

	path := filepath.Join(m.dataDir, name)
	coll, err := zvec.CreateAndOpen(path, schema, nil)
	if err != nil {
		return nil, err
	}
	m.colls[name] = coll
	return coll, nil
}

// Get returns an open collection by name.
func (m *CollectionManager) Get(name string) (*zvec.Collection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.colls[name]
	return c, ok
}

// Drop closes and permanently deletes a collection.
func (m *CollectionManager) Drop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.colls[name]
	if !ok {
		return errCollectionNotFound
	}
	if err := c.Destroy(); err != nil {
		return err
	}
	delete(m.colls, name)
	return nil
}

// List returns metadata for all open collections.
func (m *CollectionManager) List() []*CollectionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*CollectionInfo, 0, len(m.colls))
	for name, c := range m.colls {
		out = append(out, m.info(name, c))
	}
	return out
}

// Count returns the number of open collections.
func (m *CollectionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.colls)
}

// Close closes every open collection.
func (m *CollectionManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.colls {
		_ = c.Close()
	}
	m.colls = make(map[string]*zvec.Collection)
}
