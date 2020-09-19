package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type CompiledWorkspace struct {
	PermLookupTable map[string]os.FileMode
	TemplateFiles   map[string]*template.Template
	RegularFiles    map[string]string
}

func compileWorkspaceTemplate(path string) *CompiledWorkspace {
	res := &CompiledWorkspace{
		TemplateFiles:   make(map[string]*template.Template),
		RegularFiles:    make(map[string]string),
		PermLookupTable: make(map[string]os.FileMode),
	}
	panicIfErr := func(err error) {
		if err != nil {
			log.Panicf("TemplateCompiler: %v", err)
		}
	}

	processFile := func(fp string) {
		relPath, err := filepath.Rel(flagTemplatePath, fp)
		panicIfErr(err)
		stat, err := os.Stat(fp)
		panicIfErr(err)
		fileMode := stat.Mode()
		if ext := filepath.Ext(fp); strings.Contains(".sh.bash.zsh.csh.run.exec.exe", ext) {
			fileMode |= 0111
		}
		res.PermLookupTable[relPath] = fileMode
		if filepath.Ext(fp) == ".tpl" {
			data, err := ioutil.ReadFile(fp)
			panicIfErr(err)
			tpl, err := ParseTpl(string(data))
			panicIfErr(err)
			res.TemplateFiles[relPath] = tpl
		} else {
			res.RegularFiles[relPath] = fp
		}

	}
	var processDirectory func(dir string)
	processDirectory = func(dir string) {
		files, err := ioutil.ReadDir(dir)
		panicIfErr(err)

		for _, file := range files {
			if file.IsDir() {
				processDirectory(filepath.Join(dir, "./", file.Name()))
			} else {
				processFile(filepath.Join(dir, "./", file.Name()))
			}
		}
	}

	processDirectory(path)
	return res
}
