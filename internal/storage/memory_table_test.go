package storage

import (
	"fmt"
	"sync"
	"testing"
)

func TestMemoryTable_BasicOperations(t *testing.T) {
	mt := NewMemoryTable(1000)

	// Test Put
	mt.Put("key1", []byte("value1"), 0, false)
	mt.Put("key2", []byte("value2"), 0, false)

	// Test Get
	entry, ok := mt.Get("key1")
	if !ok {
		t.Fatal("key1 not found")
	}
	if string(entry.Value) != "value1" {
		t.Fatalf("Expected 'value1', got '%s'", string(entry.Value))
	}

	// Test Size
	size := mt.Size()
	if size == 0 {
		t.Fatal("Size should be > 0")
	}

	// Test GetAll
	entries := mt.GetAll()
	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}
}

func TestMemoryTable_ConcurrentWrites(t *testing.T) {
	mt := NewMemoryTable(10000)

	var wg sync.WaitGroup
	numGoroutines := 100
	opsPerGoroutine := 1000

	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := fmt.Sprintf("key_%d_%d", goroutineID, i)
				val := []byte(fmt.Sprintf("value_%d_%d", goroutineID, i))
				mt.Put(key, val, 0, false)
			}
		}(g)
	}

	wg.Wait()

	// Verify count
	entries := mt.GetAll()
	expected := numGoroutines * opsPerGoroutine
	if len(entries) != expected {
		t.Fatalf("Expected %d entries, got %d", expected, len(entries))
	}
}

func TestMemoryTable_ConcurrentReadsAndWrites(t *testing.T) {
	mt := NewMemoryTable(10000)

	// Pre-populate
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		val := []byte(fmt.Sprintf("value%d", i))
		mt.Put(key, val, 0, false)
	}

	var wg sync.WaitGroup

	// Start readers
	for r := 0; r < 50; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10000; i++ {
				key := fmt.Sprintf("key%d", i%1000)
				mt.Get(key)
			}
		}()
	}

	// Start writers
	for w := 0; w < 50; w++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				key := fmt.Sprintf("key_new_%d_%d", writerID, i)
				val := []byte(fmt.Sprintf("value_new_%d_%d", writerID, i))
				mt.Put(key, val, 0, false)
			}
		}(w)
	}

	wg.Wait()

	// Verify we can still read
	entry, ok := mt.Get("key0")
	if !ok {
		t.Fatal("key0 not found after concurrent operations")
	}
	if string(entry.Value) != "value0" {
		t.Fatalf("Expected 'value0', got '%s'", string(entry.Value))
	}
}

func BenchmarkMemoryTable_Put_Sequential(b *testing.B) {
	mt := NewMemoryTable(1000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		val := []byte("testvalue1234567890")
		mt.Put(key, val, 0, false)
	}
}

func BenchmarkMemoryTable_Put_Parallel(b *testing.B) {
	mt := NewMemoryTable(1000000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i)
			val := []byte("testvalue1234567890")
			mt.Put(key, val, 0, false)
			i++
		}
	})
}

func BenchmarkMemoryTable_Get_Parallel(b *testing.B) {
	mt := NewMemoryTable(1000000)

	// Pre-populate
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key%d", i)
		val := []byte("testvalue1234567890")
		mt.Put(key, val, 0, false)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%10000)
			mt.Get(key)
			i++
		}
	})
}

func BenchmarkMemoryTable_MixedWorkload(b *testing.B) {
	mt := NewMemoryTable(1000000)

	// Pre-populate
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key%d", i)
		val := []byte("testvalue1234567890")
		mt.Put(key, val, 0, false)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 < 7 {
				// 70% reads
				key := fmt.Sprintf("key%d", i%10000)
				mt.Get(key)
			} else {
				// 30% writes
				key := fmt.Sprintf("key_new_%d", i)
				val := []byte("testvalue1234567890")
				mt.Put(key, val, 0, false)
			}
			i++
		}
	})
}
