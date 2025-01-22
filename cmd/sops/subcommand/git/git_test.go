package git

import (
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/require"
)

func TestGetRepository_success(t *testing.T) {
	usingTempRepository(t, func(repo *git.Repository) {
		require.NotNil(t, repo)
	})
}

func TestGetRepository_error(t *testing.T) {
	usingTempDirectory(t, func() {
		repo, err := getRepository()
		require.Error(t, err)
		require.Nil(t, repo)
	})
}

func TestGetStagedBlob_success(t *testing.T) {
	usingTempRepository(t, func(repo *git.Repository) {
		objPath := "somePath"
		want := "hope this works"
		writeFile(t, objPath, want)
		blob, err := getBlobFromStaging(objPath)
		require.NoError(t, err)
		require.Nil(t, blob) // not yet staged

		gitAdd(t, objPath)

		blob, err = getBlobFromStaging(objPath)
		require.NoError(t, err)
		require.NotNil(t, blob)
		blobReader, err := blob.Reader()
		require.NoError(t, err)
		defer blobReader.Close()
		require.NotNil(t, blobReader)
		got, err := io.ReadAll(blobReader)
		require.NoError(t, err)
		require.Equal(t, want, string(got))
	})
}

func TestGetStagedBlob_NotFound(t *testing.T) {
	usingTempRepository(t, func(repo *git.Repository) {
		objPath := "somePath"
		want := "hope this works"
		writeFile(t, objPath, want)

		blob, err := getBlobFromStaging("somePath")
		require.NoError(t, err)
		require.Nil(t, blob)
	})
}

func gitAdd(t *testing.T, objPath string) {
	t.Helper()
	cmd := exec.Command("git", "add", objPath)
	output, err := cmd.Output()
	require.NoError(t, err)
	log.Debug(output)
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
}

func usingTempDirectory(t *testing.T, f func()) {
	t.Helper()
	dir := t.TempDir()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { os.Chdir(originalDir) }()
	err = os.Chdir(dir)
	require.NoError(t, err)
	f()
}

func usingTempRepository(t *testing.T, f func(repo *git.Repository)) {
	t.Helper()
	usingTempDirectory(t, func() {
		err := os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
		require.NoError(t, err)
		defer os.Unsetenv("GIT_CONFIG_GLOBAL")

		cmd := exec.Command("git", "init")
		_, err = cmd.Output()
		require.NoError(t, err)

		repo, err := getRepository()
		require.NoError(t, err)
		require.NotNil(t, repo)
		f(repo)
	})
}
