package integration_tests

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/getsops/sops/v3/decrypt"
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
	pwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("error getting pwd: %w", err))
	}
	resPath, err = filepath.Abs(filepath.Join(pwd, "res"))
	if err != nil {
		panic(fmt.Errorf("error getting resource path: %w", err))
	}

	code := m.Run()

	os.Exit(code)
}

var variousFormats = []struct {
	inferredPath string
	explicitPath string
	format       string
	content      string
}{
	{"test.enc-inferred.bin", "test.binary.enc", "binary", "foobar"},
	{"test.enc-inferred.env", "test.dotenv.enc", "dotenv", "foo=bar\n"},
	{"test.enc-inferred.ini", "test.ini.enc", "ini", "[foo]\nbar = baz\n"},
	{"test.enc-inferred.json", "test.json.enc", "json", "{\n	\"foo\": \"bar\"\n}"},
	{"test.enc-inferred.yml", "test.yaml.enc", "yaml", "foo: bar\n"},
}

func TestIntegrationStageFileThenEdit(t *testing.T) {
	usingIntegrationTempRepository(t, func() {
		obj := variousFormats[1]
		objPath := obj.inferredPath
		original := []byte(obj.content)
		writeFile(t, objPath, original)

		gitAdd(t, objPath)

		staged := gitCatCleanFile(t, ":"+objPath)
		decrypted, err := decrypt.Data(staged, obj.format)
		require.NoError(t, err)
		require.Equal(t, original, decrypted)

		working := readFile(t, objPath)
		require.Equal(t, original, working)

		writeFile(t, objPath, []byte(obj.content+"cat=meow\n"))

		output := gitDiff(t)
		log.Debug(output)
	})

}

func TestIntegrationStageFileInferredFormat(t *testing.T) {
	for _, tt := range variousFormats {
		t.Run(tt.format, func(t *testing.T) {
			usingIntegrationTempRepository(t, func() {
				objPath := tt.inferredPath
				original := []byte(tt.content)
				writeFile(t, objPath, original)

				gitAdd(t, objPath)

				staged := gitCatCleanFile(t, ":"+objPath)
				decrypted, err := decrypt.Data(staged, tt.format)
				require.NoError(t, err)
				require.Equal(t, original, decrypted)

				working := readFile(t, objPath)
				require.Equal(t, original, working)
			})
		})
	}
}

func TestIntegrationStageFileExplicitFormat(t *testing.T) {
	for _, tt := range variousFormats {
		t.Run(tt.format, func(t *testing.T) {
			usingIntegrationTempRepository(t, func() {
				objPath := tt.explicitPath
				original := []byte(tt.content)
				writeFile(t, objPath, original)

				gitAdd(t, objPath)

				staged := gitCatCleanFile(t, ":"+objPath)
				decrypted, err := decrypt.Data(staged, tt.format)
				require.NoError(t, err)
				require.Equal(t, original, decrypted)

				working := readFile(t, objPath)
				require.Equal(t, original, working)
			})
		})
	}
}

func usingIntegrationTempRepository(t *testing.T, f func()) {
	t.Helper()
	usingTempDirectory(t, func() {
		t.Helper()
		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", fmt.Sprintf("%s:%s", resPath, oldPath))
		t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")

		cmd := exec.Command("git", "init")
		_, err := cmd.Output()
		require.NoError(t, err, fmt.Sprintf("oopsie town: path: %s", os.Getenv("PATH")))

		const gitAttributes string = `
*.enc-inferred.* diff=sops-diff filter=sops-filter-inferred
*.binary.enc diff=sops-diff filter=sops-filter-binary
*.dotenv.enc diff=sops-diff filter=sops-filter-dotenv
*.ini.enc diff=sops-diff filter=sops-filter-ini
*.json.enc diff=sops-diff filter=sops-filter-json
*.yaml.enc diff=sops-diff filter=sops-filter-yaml
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

[user]
	email = <>
	name = sops test
`

		writeFile(t, ".gitattributes", []byte(gitAttributes))
		writeFile(t, ".git/config", []byte(gitConfig))
		gitAdd(t, ".gitattributes")
		gitAdd(t, ".git/config")
		gitCommit(t, "init configs for testing")

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

func gitCommit(t *testing.T, msg string) {
	t.Helper()
	cmd := exec.Command("git", "commit", "-m", msg)
	output, err := cmd.Output()
	require.NoError(t, err)
	log.Debug(output)
}

func gitDiff(t *testing.T) []byte {
	t.Helper()
	cmd := exec.Command("git", "diff")
	output, err := cmd.Output()
	require.NoError(t, err)
	log.Debug(output)
	return output
}

func gitCatCleanFile(t *testing.T, object string) []byte {
	t.Helper()
	cmd := exec.Command("git", "cat-file", "-p", object)
	output, err := cmd.Output()
	require.NoError(t, err)
	return output
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	err := os.WriteFile(path, content, 0644)
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
