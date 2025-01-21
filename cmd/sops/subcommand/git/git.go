package git

import (
	"fmt"
	"io"
	"os"

	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/getsops/sops/v3/logging"

	"github.com/go-git/go-git/plumbing"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

func init() {
	log = logging.NewLogger("GIT")
}

type GitArea int

const (
	Staging GitArea = iota
	Head
)

func getRepository() (*gogit.Repository, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	return gogit.PlainOpen(pwd)
}

func getWorktree() (*gogit.Worktree, error) {
	r, err := getRepository()
	if err != nil {
		return nil, fmt.Errorf("failed to load repository: %w", err)
	}
	return r.Worktree()
}

// Returns the staged object
// If the object is not staged, it returns nil
func getBlobFromStaging(path string) (*object.Blob, error) {
	repo, err := getRepository()
	if err != nil {
		return nil, fmt.Errorf("failed to load repository: %w", err)
	}
	index, err := repo.Storer.Index()
	if err != nil {
		return nil, fmt.Errorf("failed to read index: %w", err)
	}
	var blob *object.Blob
	for _, entry := range index.Entries {
		if entry.Name == path {
			blob, err = repo.BlobObject(entry.Hash)
			if err != nil {
				return nil, fmt.Errorf("failed to get blob object: %w", err)
			}
			return blob, nil
		}
	}
	// TODO: check for moves with getFileStatus
	return nil, nil
}

// Returns the object at the HEAD commit
// If the object is not committed, it returns nil
func getBlobFromHead(path string) (*object.Blob, error) {
	repo, err := getRepository()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get head: %w", err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		if err == plumbing.ErrObjectNotFound {
			// no commits
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	file, err := commit.File(path)
	if err != nil {
		if err == object.ErrFileNotFound {
			// file not committed
			// TODO: check for moves with getObjectStatus
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	return &file.Blob, err
}

// Returns the status of the object in the worktree
func getObjectStatus(path string) (*gogit.FileStatus, error) {
	worktree, err := getWorktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get git worktree: %w", err)
	}
	status, err := worktree.Status()
	if err != nil {
		return nil, err
	}
	return status.File(path), nil
}

// Returns the content of the blob without decrypting it
func getCleanContent(path string, gitArea GitArea) ([]byte, error) {
	var blob *object.Blob
	var err error
	switch gitArea {
	case Staging:
		blob, err = getBlobFromStaging(path)
	case Head:
		blob, err = getBlobFromHead(path)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get blob: %w", err)
	}
	if blob == nil {
		return nil, nil
	}
	blobReader, err := blob.Reader()
	if err != nil {
		return nil, fmt.Errorf("failed to get blob reader: %w", err)
	}
	defer blobReader.Close()
	content, err := io.ReadAll(blobReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read blob: %w", err)
	}
	return content, nil
}

// Returns the decrypted content
func getSmudgedContent(cleanContent []byte, path string, inputFormat string) ([]byte, error) {
	smudgedContent, err := decrypt.DataWithFormat(cleanContent, formats.FormatForPathOrString(path, inputFormat))
	if err != nil {
		return nil, err
	}
	return smudgedContent, nil
}
