package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eternal-flame-AD/batch-vs-runner/workerpool"
)

var (
	flagNProcess         int
	flagTemplatePath     string
	flagBatchStart       int
	flagBatchEnd         int
	flagBatchSize        int
	flagWorkSpaceOnly    bool
	flagWorkSpaceExec    string
	flagWorkerStartDelay int
	flagVerbose          bool

	molFileList []string
	molFileExt  string
)

var lineBreak = "\r\n"

func init() {
	// parse args and stuff

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s: batch-vs-runner [FLAGS] [SD|MOL2|PDB|DIRECTORY]...\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.IntVar(&flagNProcess, "np", 1, "no. of worker processes")
	flag.IntVar(&flagWorkerStartDelay, "delay", 0, "delay a certain amount of time (in ms) between spawning the next process, useful for programs that periodically do heavy IO")

	flag.StringVar(&flagTemplatePath, "workspace", ".", "path to job setup files (can be a directory or single file)")
	flag.StringVar(&flagWorkSpaceExec, "exec", "./job.sh", "command to execute in worker")
	flag.IntVar(&flagBatchStart, "batchStart", 1, "start from Nth molecule (cumulative across all input files)")
	flag.IntVar(&flagBatchEnd, "batchEnd", 0, "end at Nth molecule, 0 means all molecules")
	flag.IntVar(&flagBatchSize, "batchSize", 100, "batch size")
	flag.BoolVar(&flagVerbose, "verbose", false, "pass through worker script output to terminal")
	flag.BoolVar(&flagWorkSpaceOnly, "workspaceOnly", false, "generate workspace only but do not execute any job, you can use anything to execute the job once the workspace has been compiled")

	lineBreakFlag := ""
	flag.StringVar(&lineBreakFlag, "lineBreak", "unix", "linebreak for output structure: unix, dos, or mac")
	switch lineBreakFlag {
	case "dos":
		lineBreak = "\r\n"
	case "unix":
		lineBreak = "\n"
	case "mac":
		lineBreak = "\r"
	}

	flag.Parse()

	if !strings.HasPrefix(flagWorkSpaceExec, ".") {
		log.Println("WARN: executable path does not start with ., only searching in PATH")
	}

	molfileArgs := flag.Args()
	for _, fn := range molfileArgs {
		providedFS, err := os.Stat(fn)
		if err != nil {
			log.Panicf("provided structure path '%s' cannot be STATed: %v", fn, err)
		}

		tryAddMolFIle := func(path string) {
			_, err := os.Stat(path)
			if err != nil {
				log.Panicf("cannot STAT mol file '%s': %v", path, err)
			}
			switch ext := strings.ToLower(filepath.Ext(path)); ext {
			case ".sdf", ".mol", ".mol2":
				if molFileExt == "" {
					molFileExt = ext
				} else if molFileExt != ext {
					log.Panicf("file '%s' does not have the same extension as previous molecules: expected %s got %s", path, molFileExt, ext)
				}
				molFileList = append(molFileList, path)
			default:
				log.Printf("file '%s' is not of extention sdf, mol, mol2 or pdb, ignoring...", path)
			}
		}
		if providedFS.IsDir() {
			fList, err := ioutil.ReadDir(fn)
			if err != nil {
				log.Panicf("cannot open dir '%s' : %v", fn, err)
			}
			for _, f := range fList {
				tryAddMolFIle(filepath.Join(fn, "./", f.Name()))
			}
		} else {
			tryAddMolFIle(fn)
		}
	}
	if len(molFileList) == 0 {
		panic("must specify at lest 1 molecule file")
	}
	log.Printf("%d mol files located successfully", len(molFileList))
}

func main() {
	log.Println("compiling workspace template")
	workSpace := compileWorkspaceTemplate(flagTemplatePath)

	log.Println("start generating job batch files")
	batches := GenerateJobWorkspaceFromFileList(molFileList, workSpace)

	if !flagWorkSpaceOnly {
		log.Println("workspace compiled successfully! spawning workers")

		wp := workerpool.NewPool(flagNProcess)
		for _, batch := range batches {
			wp.SubmitTask(BatchExecution(batch))
		}

		wp.Start(time.Millisecond * time.Duration(flagWorkerStartDelay))
		wp.Wait()
	}
}
