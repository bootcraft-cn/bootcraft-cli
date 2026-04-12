package commands

import (
	"fmt"

	"github.com/bootcraft-cn/cli/internal/version"
)

func VersionCommand() {
	fmt.Printf("bootcraft %s (%s)\n", version.Version, version.Commit)
}
