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

	t.Run("Custom File Permission Option", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "filestore_test_perm_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		customPerm := os.FileMode(0o600)
		store, err := NewFileStore[mockUser](tmpDir, WithFilePerm[mockUser](customPerm))
		if err != nil {
			t.Fatalf("failed to create store with custom perm: %v", err)
		}

		user := mockUser{ID: 5, Name: "Frank", Email: "frank@example.com"}
		err = store.Store("frank", user)
		if err != nil {
			t.Fatalf("failed to store: %v", err)
		}

		filePath := filepath.Join(tmpDir, "frank.json")
		info, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}

		if info.Mode().Perm() != customPerm {
			t.Errorf("expected file permission to be %O, got %O", customPerm, info.Mode().Perm())
		}
	})

	t.Run("Dir and Path methods", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "filestore_test_dirpath_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		store, err := NewFileStore[mockUser](tmpDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		if store.Dir() != tmpDir {
			t.Errorf("Dir() expected %s, got %s", tmpDir, store.Dir())
		}

		expectedPath := filepath.Join(tmpDir, "test.json")
		if store.Path("test") != expectedPath {
			t.Errorf("Path() expected %s, got %s", expectedPath, store.Path("test"))
		}
	})

	t.Run("List and Delete methods", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "filestore_test_listdelete_*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		store, err := NewFileStore[mockUser](tmpDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		list, err := store.List()
		if err != nil {
			t.Fatalf("List() failed: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("expected empty list, got %d items", len(list))
		}

		u1 := mockUser{ID: 10, Name: "User1"}
		u2 := mockUser{ID: 20, Name: "User2"}
		if err := store.Store("u1", u1); err != nil {
			t.Fatalf("failed to store u1: %v", err)
		}
		if err := store.Store("u2", u2); err != nil {
			t.Fatalf("failed to store u2: %v", err)
		}

		list, err = store.List()
		if err != nil {
			t.Fatalf("List() failed: %v", err)
		}
		if len(list) != 2 {
			t.Errorf("expected list length 2, got %d", len(list))
		}

		items := make(map[int]string)
		for _, item := range list {
			items[item.ID] = item.Name
		}
		if items[10] != "User1" || items[20] != "User2" {
			t.Errorf("mismatch list items, got %+v", items)
		}

		if err := store.Delete("u1"); err != nil {
			t.Fatalf("Delete(u1) failed: %v", err)
		}

		if _, err := os.Stat(store.Path("u1")); !os.IsNotExist(err) {
			t.Errorf("expected u1.json to be deleted, stat error: %v", err)
		}

		list, err = store.List()
		if err != nil {
			t.Fatalf("List() failed: %v", err)
		}
		if len(list) != 1 {
			t.Errorf("expected list length 1, got %d", len(list))
		}
		if list[0].ID != 20 {
			t.Errorf("expected remaining item to be User2 (ID 20), got ID %d", list[0].ID)
		}

		if err := store.Delete("u1"); err == nil {
			t.Error("expected error deleting non-existent file, got nil")
		}
	})
}
