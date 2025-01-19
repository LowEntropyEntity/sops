package git

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/go-git/go-git/plumbing"
	"github.com/go-git/go-git/utils/ioutil"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type CleanOpts struct {
	ObjectPath     string
	InputFormat    string
	OutputFormat   string
	Stdin          []byte
	EncryptedStdin []byte
}

func getRepository() (*git.Repository, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	return git.PlainOpen(pwd)
}

func getWorktree() (*git.Worktree, error) {
	r, err := getRepository()
	if err != nil {
		return nil, fmt.Errorf("failed to load repository: %w", err)
	}
	return r.Worktree()
}

func getStagedBlob(path string) (*object.Blob, error) {
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
	return nil, fmt.Errorf("failed to find staged object: %w", err)
}

// returns the object at the head commit
// if the file does not exist (is not committed), it returns nil
func getHeadBlob(path string) (*object.Blob, error) {
	repo, err := getRepository()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	head, err := repo.Head()
	if err != nil {
		// TODO: consider new repository with no commits: maybe should return nil, nil?
		return nil, fmt.Errorf("failed to get head: %w", err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		if err == plumbing.ErrObjectNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	file, err := commit.File(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	return &file.Blob, err
}

func getFileStatus(path string) (*git.FileStatus, error) {
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
func headHasContent(path string) (bool, error) {
	blob, err := getHeadBlob(path)
	if err != nil {
		return false, err
	}
	return blob != nil, nil
}

func stagedHasContent(path string) (bool, error) {
	fileStatus, err := getFileStatus(path)
	if err != nil {
		return false, err
	}
	switch fileStatus.Staging {
	case git.Added:
		fallthrough
	case git.Copied:
		fallthrough
	case git.Modified:
		return true, nil
	case git.Untracked:
		fallthrough
	case git.Unmodified:
		return false, nil
	case git.Renamed:
		panic("not implemented - renamed")
	case git.Deleted:
		panic("not implemented - deleted")
	default:
		panic(fmt.Sprintf("not implemented - %s", string(fileStatus.Staging)))
	}
}

// TODO: document this
func cleanAgainstHead(path string, opts CleanOpts) (bool, []byte, error) {
	headHasContent, err := headHasContent(path)
	if err != nil {
		return false, nil, err
	}
	if !headHasContent {
		return false, nil, nil
	}
	head, err := getHeadBlob(path)
	if err != nil {
		return false, nil, err
	}
	return cleanAgainstBlob(head, path, opts)
}

// TODO: document this
func cleanAgainstStaging(path string, opts CleanOpts) (bool, []byte, error) {
	stagedHasContent, err := stagedHasContent(path)
	if err != nil {
		return false, nil, err
	}
	if !stagedHasContent {
		return false, nil, nil
	}
	staging, err := getStagedBlob(path)
	if err != nil {
		return false, nil, err
	}
	return cleanAgainstBlob(staging, path, opts)
}

// TODO: document this
func cleanAgainstBlob(blob *object.Blob, path string, opts CleanOpts) (bool, []byte, error) {
	blobReader, err := blob.Reader()
	defer ioutil.CheckClose(blobReader, &err)
	cleanBlobContent, err := io.ReadAll(blobReader)
	if err != nil {
		return false, nil, err
	}
	smudgedBlobContent, err := decrypt.DataWithFormat(cleanBlobContent, formats.FormatForPathOrString(path, opts.InputFormat))
	if err != nil {
		return false, nil, err
	}
	if bytes.Equal(smudgedBlobContent, opts.Stdin) {
		return true, cleanBlobContent, nil
	}
	return false, nil, nil
}

func Clean(path string, opts CleanOpts) error {
	changed, newCleanContent, err := cleanAgainstStaging(path, opts)
	if err != nil {
		return fmt.Errorf("failed to compare with staging: %w", err)
	}
	if !changed {
		changed, newCleanContent, err = cleanAgainstHead(path, opts)
		if err != nil {
			return fmt.Errorf("failed to compare with head: %w", err)
		}
	}
	if changed {
		_, err := os.Stdout.Write(newCleanContent)
		if err != nil {
			return fmt.Errorf("failed to write to stdout: %w", err)
		}
	} else {
		_, err := os.Stdout.Write(opts.EncryptedStdin)
		if err != nil {
			return fmt.Errorf("failed to write to stdout: %w", err)
		}
	}
	return nil
}
