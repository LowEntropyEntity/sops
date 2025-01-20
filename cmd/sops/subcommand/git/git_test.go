package git

import (
	"errors"
	"io"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/utils/ioutil"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
)

type mockGitRepository struct {
	repo *gogit.Repository
	err  error
}

func (m mockGitRepository) GetGitRepo(dir string) (*gogit.Repository, error) {
	return m.repo, m.err
}

func TestGetRepository_success(t *testing.T) {
	oldGitProvider := gitProvider
	defer func() { gitProvider = oldGitProvider }()

	mockRepo, _ := gogit.Init(memory.NewStorage(), nil)
	gitProvider = mockGitRepository{repo: mockRepo, err: nil}

	repo, err := getRepository()
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Fatal("nil repository")
	}
}

func TestGetRepository_error(t *testing.T) {
	oldGitProvider := gitProvider
	defer func() { gitProvider = oldGitProvider }()

	gitProvider = mockGitRepository{
		repo: nil,
		err:  errors.New("mock error"),
	}

	_, err := getRepository()
	if err == nil {
		t.Fatal("should return an error")
	}
}

func TestGetStagedBlob_success(t *testing.T) {
	oldGitProvider := gitProvider
	defer func() { gitProvider = oldGitProvider }()

	fs := memfs.New()
	storage := memory.NewStorage()
	r, _ := gogit.Init(storage, fs)
	gitProvider = mockGitRepository{repo: r, err: nil}
	w, _ := r.Worktree()
	want := "hope this works"
	writeObject(&fs, "some/path", want)

	blob, err := getBlobFromStaging("some/path")
	if err != nil || blob != nil {
		t.Fatal(err)
	}
	w.Add("some/path")

	blob, _ = getBlobFromStaging("some/path")
	blobReader, err := blob.Reader()
	defer ioutil.CheckClose(blobReader, &err)
	got, err := io.ReadAll(blobReader)
	if err != nil {
		t.Fatal("failed to read blob content")
	}
	if string(got) != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestGetStagedBlob_NotFound(t *testing.T) {
	oldGitProvider := gitProvider
	defer func() { gitProvider = oldGitProvider }()

	mockRepo, _ := gogit.Init(memory.NewStorage(), nil)
	gitProvider = mockGitRepository{repo: mockRepo, err: nil}

	blob, err := getBlobFromStaging("some/path")
	if err != nil {
		t.Fatal("should not return an error")
	}
	if blob != nil {
		t.Fatal("blob should be nil")
	}
}

func writeObject(fs *billy.Filesystem, path string, content string) error {
	file, err := (*fs).Create(path)
	if err != nil {
		return err
	}
	file.Write([]byte(content))
	return nil
}
