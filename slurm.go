package main

import (
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type SlurmAllocation struct {
	Nodes []SlurmNode
}

type SlurmNode struct {
	HostName string
	NTasks   int
}

var NodeNTasksComplexRegexp = regexp.MustCompile(`(\d+)\(x(\d+)\)`)

func GetSlurmInfo() *SlurmAllocation {
	envTasksPerNode := flagSlurmTaskPerNode
	envNTasks := os.Getenv("SLURM_NTASKS")
	envNNodes := os.Getenv("SLURM_NNODES")
	envNodeList := os.Getenv("SLURM_JOB_NODELIST")

	if envTasksPerNode == "" && envNTasks == "" && envNNodes == "" && envNodeList == "" {
		// slurm not active
		return nil
	}

	hostnameCmd := exec.Command("scontrol", "show", "hostnames", envNodeList)
	hostnameCmd.Stderr = os.Stderr
	hostNameBytesRead, err := hostnameCmd.CombinedOutput()
	if err != nil {
		log.Printf("cannot call scontrol: %v", err)
	}

	hostNames := strings.Fields(string(hostNameBytesRead))

	alloc := &SlurmAllocation{
		Nodes: make([]SlurmNode, len(hostNames)),
	}
	for nodeIdx := range hostNames {
		alloc.Nodes[nodeIdx].HostName = hostNames[nodeIdx]
	}

	curNodeIdx := 0
	for _, nodeNTasks := range strings.Split(flagSlurmTaskPerNode, ",") {
		if grp := NodeNTasksComplexRegexp.FindStringSubmatch(nodeNTasks); len(grp) == 3 {
			nTasksStr, multipleCntStr := grp[1], grp[2]
			nTasks, _ := strconv.Atoi(nTasksStr)
			multipleCnt, _ := strconv.Atoi(multipleCntStr)
			for i := 0; i < multipleCnt; i++ {
				alloc.Nodes[curNodeIdx].NTasks = nTasks
				curNodeIdx++
			}

		} else if nTasks, err := strconv.Atoi(nodeNTasks); err != nil {
			alloc.Nodes[curNodeIdx].NTasks = nTasks
			curNodeIdx++
		} else {
			log.Fatalf("cannot interpret SLURM_TASKS_PER_NODE or override: unknown definition %s", nodeNTasks)
		}
	}

	return alloc
}
