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

func TestIncrement(t *testing.T) {
    c := New()
    defer c.Stop()

    // Increment on missing key returns 0 and does not create entry
    result := c.Increment("counter")
    if result != 0 {
        t.Errorf("expected 0 for missing key, got %d", result)
    }
    _, exists := c.Get("counter")
    if exists {
        t.Error("Increment on missing key should not create entry")
    }

    // Set initial value then increment
    c.Set("counter", 1, 1*time.Minute)
    result = c.Increment("counter")
    if result != 2 {
        t.Errorf("expected 2 after first increment, got %d", result)
    }
    result = c.Increment("counter")
    if result != 3 {
        t.Errorf("expected 3 after second increment, got %d", result)
    }

    // Verify TTL is preserved (key still accessible after increments)
    val, ok := c.Get("counter")
    if !ok {
        t.Fatal("expected counter to still exist after increments")
    }
    if val.(int) != 3 {
        t.Errorf("expected value 3, got %v", val)
    }

    // Increment on expired key returns 0
    c.Set("expiring", 5, 50*time.Millisecond)
    time.Sleep(100 * time.Millisecond)
    result = c.Increment("expiring")
    if result != 0 {
        t.Errorf("expected 0 for expired key, got %d", result)
    }
}
