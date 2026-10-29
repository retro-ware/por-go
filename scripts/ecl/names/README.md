# ECL variable names

`cmd/eclscript` reads these when it writes `../area<N>.txt`:
`global.txt` names words for every area, `area<N>.txt` names one area's
words (over global) and `labels/area<N>.txt` its jump labels. Lines are `XXXX Name  # evidence`: the script
address in hex, a PascalCase name, an optional comment.
