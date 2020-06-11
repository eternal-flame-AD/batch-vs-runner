package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type BatchDefinition struct {
	WorkSpaceDir       string
	CumulativeStartIdx int
	CumulativeEndIdx   int
	Molecules          []MolRange
}

type MolRange struct {
	FileName  string
	StartLine int64
	EndLine   int64
}

func BatchExecution(batch BatchDefinition) func() {
	return func() {
		cmd := exec.Command("/bin/bash", "-c", flagWorkSpaceExec)
		cmd.Dir = batch.WorkSpaceDir

		if flagVerbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		} else {
			cmd.Stderr = nil
			cmd.Stdout = nil
		}

		if err := cmd.Run(); err != nil {
			log.Printf("an error while running in workspace %s: %v", batch.WorkSpaceDir, err)
		}
	}

}

func GenerateJobWorkspaceFromFileList(list []string, workspaceDef *CompiledWorkspace) []BatchDefinition {
	panicIfErr := func(err error) {
		if err != nil {
			log.Panicf("BatchBuilder: %v", err)
		}
	}

	molCounter := 0

	batchDefinitions := make([]BatchDefinition, 0)
	currentBatch := bytes.NewBufferString("")
	currentBatchDefinition := new(BatchDefinition)
	currentBatchDefinition.CumulativeStartIdx = flagBatchStart

	makeBatch := func() {
		currentBatchDefinition.CumulativeEndIdx = molCounter
		if molCounter > flagBatchEnd {
			currentBatchDefinition.CumulativeEndIdx = flagBatchEnd
		}

		if currentBatchDefinition.CumulativeEndIdx < currentBatchDefinition.CumulativeStartIdx {
			return
		}

		currentBatchDefinition.WorkSpaceDir = fmt.Sprintf("job_%d_%d", currentBatchDefinition.CumulativeStartIdx, currentBatchDefinition.CumulativeEndIdx)
		panicIfErr(os.MkdirAll(currentBatchDefinition.WorkSpaceDir, 0755))

		panicIfErr(
			ioutil.WriteFile(filepath.Join(currentBatchDefinition.WorkSpaceDir, "./job"+molFileExt), currentBatch.Bytes(), 0644),
		)

		matchDefJSON, err := json.MarshalIndent(currentBatchDefinition, "", "\t")
		panicIfErr(err)
		panicIfErr(
			ioutil.WriteFile(filepath.Join(currentBatchDefinition.WorkSpaceDir, "./jobdef.json"), matchDefJSON, 0644),
		)

		for path, tpl := range workspaceDef.TemplateFiles {
			realPath := filepath.Join(currentBatchDefinition.WorkSpaceDir, "./", path)
			os.MkdirAll(filepath.Dir(realPath), 0755)
			f, err := os.OpenFile(realPath[:len(realPath)-len(".tpl")], os.O_CREATE|os.O_WRONLY, workspaceDef.PermLookupTable[path])
			panicIfErr(err)

			if err := tpl.Execute(f, currentBatchDefinition); err != nil {
				panicIfErr(fmt.Errorf("error executing template for %s: %v", path, err))
			}
			f.Close()
		}
		for path, fp := range workspaceDef.RegularFiles {
			realPath := filepath.Join(currentBatchDefinition.WorkSpaceDir, "./", path)
			os.MkdirAll(filepath.Dir(realPath), 0755)
			f, err := os.OpenFile(realPath, os.O_CREATE|os.O_WRONLY, workspaceDef.PermLookupTable[path])
			panicIfErr(err)

			fOrig, err := os.Open(fp)
			panicIfErr(err)
			_, err = io.Copy(f, fOrig)
			if err != nil {
				panic(err)
			}
			fOrig.Close()
			f.Close()
		}

		batchDefinitions = append(batchDefinitions, *currentBatchDefinition)
		currentBatchDefinition = new(BatchDefinition)
		currentBatch = bytes.NewBufferString("")
	}

	appendStructure := func(s string, r MolRange) {
		molCounter++
		if molCounter < flagBatchStart {
			return
		}

		currentBatchDefinition.Molecules = append(currentBatchDefinition.Molecules, r)
		currentBatch.WriteString(s)
		currentBatch.WriteString("$$$$")
		currentBatch.WriteString(lineBreak)
		if molCounter-currentBatchDefinition.CumulativeStartIdx+1 == flagBatchSize {
			makeBatch()
			currentBatchDefinition.CumulativeStartIdx = molCounter + 1
		}
	}

	for _, f := range list {
		fHandle, err := os.Open(f)
		panicIfErr(err)
		reader := bufio.NewScanner(fHandle)

		currentStructure := bytes.NewBufferString("")
		startLineIdx := int64(1)
		currentLineIdx := int64(0)
		for reader.Scan() {
			currentLineIdx++
			line := reader.Text()
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			if line == "$$$$" {

				appendStructure(currentStructure.String(), MolRange{
					FileName:  f,
					StartLine: startLineIdx,
					EndLine:   currentLineIdx,
				})
				if molCounter == flagBatchEnd {
					break
				}

				startLineIdx = currentLineIdx + 1
				currentStructure = bytes.NewBufferString("")
			} else {
				currentStructure.WriteString(line)
				currentStructure.WriteString(lineBreak)
			}
		}
		if currentStructure.Len() > 8 {
			currentStructure.WriteString(lineBreak)

			appendStructure(currentStructure.String(), MolRange{
				FileName:  f,
				StartLine: startLineIdx,
				EndLine:   currentLineIdx,
			})
			if molCounter == flagBatchEnd {
				break
			}
		}
		fHandle.Close()
	}

	if molCounter-currentBatchDefinition.CumulativeStartIdx+1 > 0 {
		makeBatch()
	}

	return batchDefinitions
}
