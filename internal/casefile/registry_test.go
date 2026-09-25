package casefile

import (
	"errors"
	"testing"
)

func TestRegistryLifecycle(t *testing.T) {
	r := NewRegistry()
	if got := r.Names(); len(got) != 0 {
		t.Fatalf("新登记表应为空，got %v", got)
	}
	c := BrackishDemo()
	if err := r.Register(c); err != nil {
		t.Fatalf("登记失败：%v", err)
	}
	if err := r.Register(c); !errors.Is(err, ErrExists) {
		t.Fatalf("重名应返回 ErrExists，got %v", err)
	}
	empty := c
	empty.Name = ""
	if err := r.Register(empty); !errors.Is(err, ErrEmptyName) {
		t.Fatalf("空名称应返回 ErrEmptyName，got %v", err)
	}
	got, err := r.Get("brackish-demo")
	if err != nil || got.Name != "brackish-demo" {
		t.Fatalf("取用工况档失败：%v %+v", err, got)
	}
	if _, err := r.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("缺失档应返回 ErrNotFound，got %v", err)
	}
}

// TestRegistriesIndependent 两个登记表实例互不串名。
func TestRegistriesIndependent(t *testing.T) {
	r1 := DefaultRegistry()
	r2 := NewRegistry()
	if _, err := r1.Get("brackish-demo"); err != nil {
		t.Fatalf("默认登记表应含内置档：%v", err)
	}
	if _, err := r2.Get("brackish-demo"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("新建登记表不应看到另一登记表的内置档，got %v", err)
	}
}
