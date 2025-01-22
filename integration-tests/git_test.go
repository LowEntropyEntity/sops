package integration_tests

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/getsops/sops/v3/logging"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

var log *logrus.Logger

func init() {
	log = logging.NewLogger("GIT")
}

var resPath string

func TestMain(m *testing.M) {
	oldEnv := os.Getenv("GIT_CONFIG_GLOBAL")
	err := os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	if err != nil {
		panic(fmt.Errorf("error setting GIT_CONFIG_GLOBAL: %w", err))
	}
	defer func() {
		os.Setenv("GIT_CONFIG_GLOBAL", oldEnv)
		if err != nil {
			panic(fmt.Errorf("error resetting GIT_CONFIG_GLOBAL: %w", err))
		}
	}()
	pwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("error getting pwd: %w", err))
	}
	resPath, err = filepath.Abs(filepath.Join(pwd, "res"))
	if err != nil {
		panic(fmt.Errorf("error getting resource path: %w", err))
	}
	oldPath := os.Getenv("PATH")
	err = os.Setenv("PATH", fmt.Sprintf("%s:%s", resPath, oldPath))
	if err != nil {
		panic(fmt.Errorf("error setting PATH: %w", err))
	}
	defer func() {
		os.Setenv("PATH", oldPath)
		if err != nil {
			panic(fmt.Errorf("error resetting PATH: %w", err))
		}
	}()

	m.Run()
}

func TestIntegrationClean_success(t *testing.T) {
	usingIntegrationTempRepository(t, func() {
		objPath := "test.enc.env"
		smudgedContent := "foo=bar"
		writeFile(t, objPath, smudgedContent)

		gitAdd(t, objPath)

		stagedContent := gitCatCleanFile(t, ":"+objPath)
		require.NotEqual(t, smudgedContent, string(stagedContent))
	})
}

func usingIntegrationTempRepository(t *testing.T, f func()) {
	t.Helper()
	usingTempDirectory(t, func() {
		t.Helper()
		err := os.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
		require.NoError(t, err)
		defer os.Unsetenv("GIT_CONFIG_GLOBAL")

		cmd := exec.Command("git", "init")
		_, err = cmd.Output()
		require.NoError(t, err, fmt.Sprintf("oopsie town: path: %s", os.Getenv("PATH")))

		const gitAttributes string = `
*.enc.* diff=sops-diff filter=sops-filter-inferred
*.enc diff=sops-diff filter=sops-filter-binary
`

		const gitConfig string = `
[diff "sops-diff"]
	textconv = sops-git-binary git diff

[filter "sops-filter-inferred"]
	smudge = sops-git-binary git smudge %f
	clean = sops-git-binary git clean %f
	required = true

[filter "sops-filter-binary"]
	smudge = sops-git-binary git smudge --input-type binary --output-type binary %f
	clean = sops-git-binary git clean --input-type binary --output-type binary %f
	required = true

[filter "sops-filter-dotenv"]
	smudge = sops-git-binary git smudge --input-type dotenv --output-type dotenv %f
	clean = sops-git-binary git clean --input-type dotenv --output-type dotenv %f
	required = true

[filter "sops-filter-ini"]
	smudge = sops-git-binary git smudge --input-type ini --output-type ini %f
	clean = sops-git-binary git clean --input-type ini --output-type ini %f
	required = true

[filter "sops-filter-json"]
	smudge = sops-git-binary git smudge --input-type json --output-type json %f
	clean = sops-git-binary git clean --input-type json --output-type json %f
	required = true

[filter "sops-filter-yaml"]
	smudge = sops-git-binary git smudge --input-type yaml --output-type yaml %f
	clean = sops-git-binary git clean --input-type yaml --output-type yaml %f
	required = true
`

		writeFile(t, ".gitattributes", gitAttributes)
		writeFile(t, ".git/config", gitConfig)

		f()
	})
}

func gitAdd(t *testing.T, objPath string) {
	t.Helper()
	cmd := exec.Command("git", "add", objPath)
	output, err := cmd.Output()
	require.NoError(t, err)
	log.Debug(output)
}

func gitCatCleanFile(t *testing.T, object string) []byte {
	t.Helper()
	cmd := exec.Command("git", "cat-file", "-p", object)
	output, err := cmd.Output()
	require.NoError(t, err)
	return output
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

	sopsConfigPath, err := os.Open(filepath.Join(resPath, ".sops.yaml"))
	require.NoError(t, err)
	defer sopsConfigPath.Close()

	destFile, err := os.Create(".sops.yaml")
	require.NoError(t, err)
	defer destFile.Close()

	_, err = io.Copy(destFile, sopsConfigPath)
	require.NoError(t, err)
	f()
}
