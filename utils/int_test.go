package utils

import "testing"

func TestIntPointers(t *testing.T) {
	t.Run("IntPointer", func(t *testing.T) {
		val := 42
		p := IntPointer(val)
		if p == nil || *p != val {
			t.Errorf("expected %d, got %v", val, p)
		}
	})

	t.Run("Int32Pointer", func(t *testing.T) {
		var val int32 = 42
		p := Int32Pointer(val)
		if p == nil || *p != val {
			t.Errorf("expected %d, got %v", val, p)
		}
	})

	t.Run("Int64Pointer", func(t *testing.T) {
		var val int64 = 42
		p := Int64Pointer(val)
		if p == nil || *p != val {
			t.Errorf("expected %d, got %v", val, p)
		}
	})

	t.Run("UintPointer", func(t *testing.T) {
		var val uint = 42
		p := UintPointer(val)
		if p == nil || *p != val {
			t.Errorf("expected %d, got %v", val, p)
		}
	})

	t.Run("Uint32Pointer", func(t *testing.T) {
		var val uint32 = 42
		p := Uint32Pointer(val)
		if p == nil || *p != val {
			t.Errorf("expected %d, got %v", val, p)
		}
	})

	t.Run("Uint64Pointer", func(t *testing.T) {
		var val uint64 = 42
		p := Uint64Pointer(val)
		if p == nil || *p != val {
			t.Errorf("expected %d, got %v", val, p)
		}
	})
}
