package helper

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"text/template"
)

//go:embed all:stubs
var TemplateFiles embed.FS // variable containing all files

func GetTemplate(name string) ([]byte, error) {
	return TemplateFiles.ReadFile("stubs/" + name)
}

func ListAllFiles() {
	err := fs.WalkDir(TemplateFiles, "stubs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			fmt.Printf("Mappe fundet: %s\n", path)
		} else {
			fmt.Printf("Fil fundet: %s\n", path)
			// The file content:
			// data, _ := TemplateFiles.ReadFile(path)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Fejl under gennemgang: %v\n", err)
	}
}

type StubDetails struct {
	Name, FileName, Destination string
	Values                      map[string]string
}

func (s *StubDetails) CreateStub() {

	//_, filename, _, _ := runtime.Caller(0)
	// 'filename' er nu den absolutte sti til den .go fil, der kører koden
	//basePath := filepath.Dir(filename)
	//fullPath := filepath.Join(basePath, "data.txt")
	//println(basePath)
	//contentsBuff, err := os.ReadFile(s.Name)

	contentsBuff, err := GetTemplate(s.Name)

	if err != nil {
		log.Fatalf("Unable to read file: %s", s.Name)
	}

	f, err := os.OpenFile(s.Destination+s.FileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		log.Fatalf("Unable to open file: %s", s.FileName)
	}
	defer f.Close()

	template, err := template.New(s.FileName).Parse(string(contentsBuff))
	if err != nil {
		log.Fatalf("Unable to parse template: %s", s.Name)
	}
	template.Execute(f, s.Values)
}

/*
How to use:

stub := StubDetails{
		Name:        "./model.go.stub",
		FileName:    "model.go",
		Destination: "./",
		Values: map[string]string{
			"Model": "User",
		},
	}

*/
