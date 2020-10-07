package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
	flagJobDirPrefix     string
	flagUseSlurm         bool
	flagSlurmTaskPerNode string

	molFileList []string
	molFileExt  string
)

var lineBreak = "\r\n"

func init() {
	// parse args and stuff

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s: batch-vs-runner [FLAGS] [SD|PDB|PDBQT|MOL2|DIRECTORY]...\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.IntVar(&flagNProcess, "np", 1, "no. of worker processes (does not apply if slurm mode is in use)")
	flag.IntVar(&flagWorkerStartDelay, "delay", 0, "delay a certain amount of time (in ms) between spawning the next process, useful for programs that periodically do heavy IO")

	flag.StringVar(&flagTemplatePath, "workspace", ".", "path to job setup files (can be a directory or single file)")
	flag.StringVar(&flagWorkSpaceExec, "exec", "./job.sh", "command to execute in worker")
	flag.IntVar(&flagBatchStart, "batchStart", 1, "start from Nth molecule (cumulative across all input files)")
	flag.IntVar(&flagBatchEnd, "batchEnd", 0, "end at Nth molecule, 0 means all molecules")
	flag.IntVar(&flagBatchSize, "batchSize", 100, "batch size")
	flag.BoolVar(&flagVerbose, "verbose", false, "pass through worker script output to terminal")
	flag.BoolVar(&flagWorkSpaceOnly, "workspaceOnly", false, "generate workspace only but do not execute any job, you can use anything to execute the job once the workspace has been compiled")
	flag.StringVar(&flagJobDirPrefix, "prefix", "job", "prefix on individual job work directory")

	flag.BoolVar(&flagUseSlurm, "enableSlurm", true, "detect slurm allocations based on environment variable and use srun to run jobs")
	flag.StringVar(&flagSlurmTaskPerNode, "slurmNodeTaskOverride", os.Getenv("SLURM_TASKS_PER_NODE"), "override how many tasks to distribute to each node from the env received from slurm")

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
}

func main() {

	batches := prepareBatches()
	log.Printf("%d batches in total", len(batches))

	if !flagWorkSpaceOnly {
		runBatches(batches)
		os.Exit(int(HadErrorFlag))
	}

}

func prepareBatches() []BatchDefinition {
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
			switch ext := strings.ToLower(filepath.Ext(strings.TrimSuffix(path, ".gz"))); ext {
			case ".sdf", ".sd", ".pdb", ".pdbqt", ".mol2":
				if molFileExt == "" {
					molFileExt = ext
				} else if molFileExt != ext {
					log.Panicf("file '%s' does not have the same extension as previous molecules: expected %s got %s", path, molFileExt, ext)
				}
				molFileList = append(molFileList, path)
			default:
				log.Printf("file '%s' is not of extention sdf, sd, mol2, pdb or pdbqt, ignoring...", path)
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

	log.Println("compiling workspace template")
	workSpace := compileWorkspaceTemplate(flagTemplatePath)

	log.Println("start generating job batch files")
	batches := GenerateJobWorkspaceFromFileList(molFileList, workSpace)
	log.Println("workspace compiled successfully!")

	return batches
}

func runBatches(batches []BatchDefinition) {

	slurmAlloc := GetSlurmInfo()
	if slurmAlloc != nil {
		log.Printf("Active slurm allocation. UseSlurmMode=%v", flagUseSlurm)
		log.Printf("%15s|%8s|", "HostName", "NTasks")
		log.Printf("%15s|%8s|", "-----", "-----")
		for _, node := range slurmAlloc.Nodes {
			log.Printf("%15s|%8d|", node.HostName, node.NTasks)
		}
	}

	runCtx, runCtxCancel := context.WithCancel(context.Background())
	defer runCtxCancel()

	if !flagUseSlurm || slurmAlloc == nil {

		wp := NewPool(flagNProcess)
		for _, batch := range batches {
			wp.SubmitTask(BatchExecution(runCtx, batch, nil))
		}

		wp.Start(time.Millisecond * time.Duration(flagWorkerStartDelay))
		wp.Wait()

	} else {

		if flagNProcess != 1 {
			log.Println("WARN: -np flag is disregarded in slurm mode.")
		}

		jobChan := make(chan BatchDefinition)
		serverWaitGroup := sync.WaitGroup{}

		nTasksTotal := 0
		// start goroutines to serve individual task slots
		for _, node := range slurmAlloc.Nodes {
			nTasksTotal += node.NTasks
			for i := 0; i < node.NTasks; i++ {
				nodeBatchProxyPrefix := []string{"srun", "-n", "1", "-w", node.HostName}
				serverWaitGroup.Add(1)
				go func() {
					defer serverWaitGroup.Done()
					for batch := range jobChan {
						BatchExecution(runCtx, batch, nodeBatchProxyPrefix)()
					}
				}()
			}
		}

		startupCounter := nTasksTotal
		// distribute jobs
		for _, batch := range batches {
			jobChan <- batch
			if startupCounter > 0 {
				time.Sleep(time.Millisecond * time.Duration(flagWorkerStartDelay))
				startupCounter--
			}
		}
		close(jobChan)

		serverWaitGroup.Wait()
	}

}
