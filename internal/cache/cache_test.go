package cache

import (
    "testing"
    "time"
)

func TestSetAndGet(t *testing.T) {
    c := New()
    defer c.Stop()
    c.Set("key1", "value1", 1*time.Minute)
    val, ok := c.Get("key1")
    if !ok {
        t.Fatal("expected key1 to exist")
    }
    if val != "value1" {
        t.Errorf("expected value1, got %v", val)
    }
}

func TestExpiration(t *testing.T) {
    c := New()
    defer c.Stop()
    c.Set("key1", "value1", 50*time.Millisecond)
    time.Sleep(100 * time.Millisecond)
    _, ok := c.Get("key1")
    if ok {
        t.Error("expected key1 to be expired")
    }
}

func TestDelete(t *testing.T) {
    c := New()
    defer c.Stop()
    c.Set("key1", "value1", 1*time.Minute)
    c.Delete("key1")
    _, ok := c.Get("key1")
    if ok {
        t.Error("expected key1 to be deleted")
    }
}

func TestGetMiss(t *testing.T) {
    c := New()
    defer c.Stop()
    _, ok := c.Get("nonexistent")
    if ok {
        t.Error("expected miss for nonexistent key")
    }
}
