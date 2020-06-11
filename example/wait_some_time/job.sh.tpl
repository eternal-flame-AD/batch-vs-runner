#!/bin/bash

START_LIGAND_IDX={{ .CumulativeStartIdx }}
END_LIGAND_IDX={{ .CumulativeEndIdx }}
echo "sleeping for ${START_LIGAND_IDX} seconds"
sleep $START_LIGAND_IDX
echo "finished sleeping for ${START_LIGAND_IDX} seconds"
