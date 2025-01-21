package git

import (
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func usingTempDirectory(t *testing.T, f func()) {
	dir := t.TempDir()
	originalDir, err := os.Getwd()
	assert.Nil(t, err)
	defer func() { os.Chdir(originalDir) }()
	err = os.Chdir(dir)
	assert.Nil(t, err)
	f()
}

func usingTempRepository(t *testing.T, f func()) {
	usingTempDirectory(t, func() {
		cmd := exec.Command("git", "init")
		_, err := cmd.Output()
		assert.Nil(t, err)
		repo, err := getRepository()
		assert.Nil(t, err)
		if repo == nil {
			t.Fatal("nil repository")
		}
		f()
	})
}

func TestGetRepository_success(t *testing.T) {
	usingTempRepository(t, func() {
		repo, err := getRepository()
		assert.Nil(t, err)
		if repo == nil {
			t.Fatal("nil repository")
		}
	})
}

func TestGetRepository_error(t *testing.T) {
	usingTempDirectory(t, func() {
		repo, err := getRepository()
		assert.Nil(t, err)
		if repo != nil {
			t.Fatal("should be nil")
		}
	})
}

func TestGetStagedBlob_success(t *testing.T) {
	usingTempRepository(t, func() {
		repo, err := getRepository()
		if err != nil {
			t.Fatal(err)
		}
		if repo == nil {
			t.Fatal("nil repository")
		}
		objPath := "somePath"
		want := "hope this works"
		err = os.WriteFile(objPath, []byte(want), 0644)
		assert.Nil(t, err)
		blob, err := getBlobFromStaging(objPath)
		if err != nil || blob != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("git", "add", objPath)
		_, err = cmd.Output()
		assert.Nil(t, err)

		blob, _ = getBlobFromStaging(objPath)
		blobReader, err := blob.Reader()
		assert.Nil(t, err)
		defer blobReader.Close()
		got, err := io.ReadAll(blobReader)
		assert.Nil(t, err)
		if string(got) != want {
			t.Fatalf("expected %s, got %s", want, got)
		}
	})
}

func TestGetStagedBlob_NotFound(t *testing.T) {
	usingTempRepository(t, func() {
		repo, err := getRepository()
		if err != nil {
			t.Fatal(err)
		}
		if repo == nil {
			t.Fatal("nil repository")
		}
		objPath := "somePath"
		want := "hope this works"
		err = os.WriteFile(objPath, []byte(want), 0644)
		assert.Nil(t, err)

		blob, err := getBlobFromStaging("somePath")
		if err != nil {
			t.Fatal("should not return an error")
		}
		if blob != nil {
			t.Fatal("blob should be nil")
		}
	})
}
