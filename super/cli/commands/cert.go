package commands

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"
	"github.com/spf13/cobra"
)

// ControllerCmd represents the Controller command
var CertCmd = &cobra.Command{
	GroupID: groups.GeneratorGroup.ID,
	Args:    cobra.MinimumNArgs(1),
	Use:     "make:cert [host]",
	Short:   "Create new certificate for a host - ex. make:cert example.com or make:cert --host locahost",
	Long:    `Create new certificate for a host - ex. make:cert example.com or make:cert --host locahost`,
	Run:     certCommand,
}

func certCommand(cmd *cobra.Command, args []string) {
	host := args[0]

	cmdEnv := exec.Command("go", "env", "GOROOT")
	var out bytes.Buffer
	cmdEnv.Stdout = &out

	if err := cmdEnv.Run(); err != nil {
		fmt.Printf("❌ Fail, try finding GOROOT by 'go env': %v\n", err)
		return
	}

	goroot := strings.TrimSpace(out.String())
	scriptPath := filepath.Join(goroot, "src", "crypto", "tls", "generate_cert.go")

	tidy := exec.Command("go", "run", scriptPath, "--host="+host)

	err := tidy.Run()
	if err != nil {
		fmt.Printf("❌ Error generating certificate: %v\n", err)
		return
	}
}
