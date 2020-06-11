Here is the template of a workspace for gold.

For files ending with .tpl like this one, it will be interpolated by the Go template system, you can use this feature to interpolate batch-specific informations (example: the start and end ID of the molecule working on in this batch).

An example, in a bash file you can do:


#!/bin/bash
START_LIGAND_IDX={{ .CumulativeStartIdx }}
END_LIGAND_IDX={{ .CumulativeEndIdx }}
echo "I am now working with ligand index ${START_LIGAND_IDX} to ${END_LIGAND_IDX}"

And if you try to generate this workspace, you will see the above variables have been interpolated automatically! You can also do this for job.sh, see wait_some_time example.

