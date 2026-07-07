# Ownership conflict overlapping file

Purpose: prove that overlapping whole-file ownership is rejected.

Represented command: `tbboot install file:conflicting-source`

Kind: `atomic-case`

Polarity: `negative`

Normative outputs: `expected/exit-code.txt`, `expected/stdout-contains.yaml`, `expected/consumer/`
