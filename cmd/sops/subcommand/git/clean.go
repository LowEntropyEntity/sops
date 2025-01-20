package git

import (
	"bytes"
	"fmt"
	"os"

	gogit "github.com/go-git/go-git/v5"
)

type CleanOpts struct {
	ObjectPath     string
	InputFormat    string
	OutputFormat   string
	Stdin          []byte
	EncryptedStdin []byte
}

func contentIsIdentical(path string, comparisonGitArea GitArea, opts CleanOpts) (bool, []byte, error) {
	objectStatus, err := getObjectStatus(path)
	if err != nil {
		return false, nil, err
	}
	switch objectStatus.Worktree {
	case gogit.Added:
		if objectStatus.Staging != gogit.Added || comparisonGitArea == Head {
			return false, nil, nil
		}
		// we're in an unmerged, both added situation
		panic("not implemented") // TODO: go by merging rules?
	case gogit.Modified:
		// let further logic decide
	case gogit.UpdatedButUnmerged:
		panic("not implemented") // TODO: figure out what to do
	default: // Untracked, Unmodified, Deleted, Renamed, Copied
		return false, nil, fmt.Errorf("we should not compare objects with status [%c]: %s", objectStatus.Worktree, path)
	}

	cleanContent, err := getCleanContent(path, comparisonGitArea)
	if err != nil {
		return false, nil, err
	}
	if cleanContent == nil {
		return false, nil, nil
	}
	smudgedContent, err := getSmudgedContent(cleanContent, opts.ObjectPath, opts.InputFormat)
	if err != nil {
		return false, nil, err
	}
	if !bytes.Equal(smudgedContent, opts.Stdin) {
		return false, nil, nil
	}
	identical := cleanContentIsEssentiallyIdentical(cleanContent, opts.EncryptedStdin, opts)
	if identical {
		return true, cleanContent, nil
	}
	return false, nil, nil
}

func cleanContentIsEssentiallyIdentical(c1 []byte, c2 []byte, opts CleanOpts) bool {
	if bytes.Equal(c1, c2) {
		return true
	}
	panic("not implemented") // TODO: figure out what to do
	// deserialize, drop the mac and lastModified, and compare
}

func Clean(path string, opts CleanOpts) error {
	stagedIsIdentical, cleanContent, err := contentIsIdentical(path, Staging, opts)
	if err != nil {
		return err
	}
	if stagedIsIdentical {
		_, err := os.Stdout.Write(cleanContent)
		if err != nil {
			return fmt.Errorf("failed to write to stdout: %w", err)
		}
		return nil
	}

	headIsIdentical, cleanContent, err := contentIsIdentical(path, Head, opts)
	if err != nil {
		return err
	}
	if headIsIdentical {
		_, err := os.Stdout.Write(cleanContent)
		if err != nil {
			return fmt.Errorf("failed to write to stdout: %w", err)
		}
		return nil
	}
	_, err = os.Stdout.Write(opts.EncryptedStdin)
	if err != nil {
		return fmt.Errorf("failed to write to stdout: %w", err)
	}

	return nil
}
