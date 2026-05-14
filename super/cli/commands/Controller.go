/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package commands

import (
	groups "github.com/nicklasjeppesen/going_internal/super/cli/groups"
	helper "github.com/nicklasjeppesen/going_internal/super/cli/helper"
	"github.com/spf13/cobra"
)

// ControllerCmd represents the Controller command
var ControllerCmd = &cobra.Command{
	GroupID: groups.GeneratorGroup.ID,
	Args:    cobra.MinimumNArgs(1),
	Use:     "Controller",
	Short:   "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		// Search in all stubs
		stub := helper.StubDetails{
			Name:        "controller/controller.go.stub",
			FileName:    name + ".go",
			Destination: "./internal/app/http/controller/",
			Values: map[string]string{
				"Model": name,
			},
		}

		stub.CreateStub()

		/*
			// Sørg for at navnet starter med stort (Go konvention)
			fileName := fmt.Sprintf("internal/app/http/controller/%s.go", strings.ToLower(name))

			// Simpel template
			content := fmt.Sprintf("package controller\n\ntype %s struct {\n}\n", name)

			// Skriv filen til disk
			err := os.WriteFile(fileName, []byte(content), 0644)
			if err != nil {
				fmt.Printf("Fejl ved oprettelse af controller: %v\n", err)
				return
			}
			fmt.Printf("Controller %s er oprettet i %s\n", name, fileName)
		*/
	},
}
