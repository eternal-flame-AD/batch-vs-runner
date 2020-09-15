#!/bin/bash

START_LIGAND_IDX={{ .CumulativeStartIdx }}
END_LIGAND_IDX={{ .CumulativeEndIdx }}
let LIGAND_COUNT=END_LIGAND_IDX-START_LIGAND_IDX+1
PDB_INPUT=input.pdb

POSE_COUNT=10



# separate ligand files and then generate conformers for each ligand
for i in $(seq 1 $LIGAND_COUNT); do
    obabel -f $i -l $i job.sdf "-Olig_$i.sdf"

    /stor/home/gf6244/CCDC/CSD_2020/bin/conformer_generator -nc 20 "lig_$i.sdf"  "lig_${i}_conformers.sdf"

    let LIGAND_IDX=$i+$START_LIGAND_IDX-1

    $ROSETTA_SCRIPTS/python/public/molfile_to_params.py --long-names -n "LIG$LIGAND_IDX" -p "lig_$i" --center=0,0,0 --conformers-in-one-file "lig_${i}_conformers.sdf"
done


# for each input, combine protein file and the ligand as a single pose
rm -f poses.txt || true
for i in lig_*.pdb
do
    if echo "$i" | grep -v 'conformer'; then
        echo "${PDB_INPUT} $i" >> poses.txt
    fi
done

sed -i "s/input.pdb/${PDB_INPUT}/g" dock.xml
mkdir .cache || true
mkdir ../.grid_cache || true
$ROSETTA_BIN/rosetta_scripts.static.linuxgccrelease @ dock.flags > rosetta.log 2> rosetta.err
rm -rf .cache || true