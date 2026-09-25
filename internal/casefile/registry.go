package casefile

import (
	"errors"
	"sort"
	"sync"
)

var (
	// ErrNotFound 点名取用的工况档不存在。
	ErrNotFound = errors.New("casefile: 未找到指定名称的工况档")
	// ErrExists 登记时名称已被占用。
	ErrExists = errors.New("casefile: 同名工况档已存在")
	// ErrEmptyName 工况档没有名称。
	ErrEmptyName = errors.New("casefile: 工况档名称不能为空")
)

// Registry 是内存中的具名工况档登记表，并发安全。
// 每个 Registry 实例彼此独立，互不串名。
type Registry struct {
	mu    sync.RWMutex
	cases map[string]CaseFile
	order []string
}

// NewRegistry 创建一个空登记表。
func NewRegistry() *Registry {
	return &Registry{cases: make(map[string]CaseFile)}
}

// Register 登记一份工况档；重名或名称为空时返回错误。
func (r *Registry) Register(c CaseFile) error {
	if c.Name == "" {
		return ErrEmptyName
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.cases[c.Name]; ok {
		return ErrExists
	}
	r.cases[c.Name] = c
	r.order = append(r.order, c.Name)
	return nil
}

// Get 按名称取用工况档。
func (r *Registry) Get(name string) (CaseFile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.cases[name]
	if !ok {
		return CaseFile{}, ErrNotFound
	}
	return c, nil
}

// Names 按登记顺序返回所有工况档名称。
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	sort.Strings(out)
	return out
}
