package git

import (
	"io"
	"os"
	"strings"

	"github.com/urfave/cli"
)

func Diff(args cli.Args) error {
	stdin := os.Stdin
	stdout := os.Stdout

	log.Debug("args: " + strings.Join(args, " "))

	_, err := io.Copy(stdout, stdin)
	return err
}
