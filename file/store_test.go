package file

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// mockUser 是一個用於測試的結構體，實作了 validator.IValidator
type mockUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (m mockUser) Validate() error {
	if m.ID <= 0 {
		return errors.New("id must be positive")
	}
	if m.Name == "" {
		return errors.New("name cannot be empty")
	}
	return nil
}

func TestFileStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filestore_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewFileStore[mockUser](tmpDir)
	if err != nil {
		t.Fatalf("failed to create NewFileStore: %v", err)
	}

	t.Run("Store and Read valid object", func(t *testing.T) {
		user := mockUser{ID: 1, Name: "Alice", Email: "alice@example.com"}
		err := store.Store("alice", user)
		if err != nil {
			t.Fatalf("failed to store: %v", err)
		}

		// 檢查檔案內容是否正確
		filePath := filepath.Join(tmpDir, "alice.json")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("expected file to exist at %s", filePath)
		}

		readUser, err := store.Read("alice")
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}

		if readUser.ID != user.ID || readUser.Name != user.Name || readUser.Email != user.Email {
			t.Errorf("read object mismatch, got: %+v, want: %+v", readUser, user)
		}
	})

	t.Run("Store invalid object (fails validation)", func(t *testing.T) {
		invalidUser := mockUser{ID: 0, Name: "Bob"} // 違反 Validate() 的 ID 規則
		err := store.Store("bob", invalidUser)
		if err == nil {
			t.Fatal("expected store to fail due to validation, but it succeeded")
		}
	})

	t.Run("Path Traversal Defense", func(t *testing.T) {
		user := mockUser{ID: 2, Name: "Charlie"}
		err := store.Store("../charlie", user)
		if err == nil {
			t.Fatal("expected path traversal to fail, but it succeeded")
		}
	})

	t.Run("File Not Found on Read", func(t *testing.T) {
		_, err := store.Read("non_existent")
		if err == nil {
			t.Fatal("expected read to fail for non-existent file, but it succeeded")
		}
	})

	t.Run("UpdateDir successfully redirects storage", func(t *testing.T) {
		tmpDir2, err := os.MkdirTemp("", "filestore_test_update_*")
		if err != nil {
			t.Fatalf("failed to create temp dir 2: %v", err)
		}
		defer os.RemoveAll(tmpDir2)

		err = store.UpdateDir(tmpDir2)
		if err != nil {
			t.Fatalf("failed to update dir: %v", err)
		}

		user := mockUser{ID: 3, Name: "Dan", Email: "dan@example.com"}
		err = store.Store("dan", user)
		if err != nil {
			t.Fatalf("failed to store after updating dir: %v", err)
		}

		// 檢查檔案是否確實寫入至新目錄，而非舊目錄
		filePath := filepath.Join(tmpDir2, "dan.json")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("expected file to exist at new directory %s", filePath)
		}

		oldFilePath := filepath.Join(tmpDir, "dan.json")
		if _, err := os.Stat(oldFilePath); err == nil {
			t.Errorf("did not expect file to exist in old directory %s", oldFilePath)
		}

		readUser, err := store.Read("dan")
		if err != nil {
			t.Fatalf("failed to read from updated dir: %v", err)
		}
		if readUser.Name != "Dan" {
			t.Errorf("expected Name to be Dan, got %s", readUser.Name)
		}
	})

	t.Run("Tilde Expansion Support", func(t *testing.T) {
		tildeDir := "~/tmp/filestore_test_tilde"
		store, err := NewFileStore[mockUser](tildeDir)
		if err != nil {
			t.Fatalf("failed to create store with tilde path: %v", err)
		}

		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("failed to get user home: %v", err)
		}
		resolvedDir := filepath.Join(home, "tmp", "filestore_test_tilde")
		defer os.RemoveAll(resolvedDir)

		user := mockUser{ID: 4, Name: "Eve", Email: "eve@example.com"}
		err = store.Store("eve", user)
		if err != nil {
			t.Fatalf("failed to store user: %v", err)
		}

		filePath := filepath.Join(resolvedDir, "eve.json")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("expected file to exist at resolved tilde path: %s", filePath)
		}

		readUser, err := store.Read("eve")
		if err != nil {
			t.Fatalf("failed to read user: %v", err)
		}
		if readUser.Name != "Eve" {
			t.Errorf("expected Name to be Eve, got %s", readUser.Name)
		}
	})
}


