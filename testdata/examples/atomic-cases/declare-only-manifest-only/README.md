# Declare only manifest only

Purpose: prove that `--declare-only` mutates only the manifest.

Represented command: `tbboot install file:local-example-source --artifact base-readme --declare-only`

Kind: `atomic-case`

Polarity: `positive`

Normative outputs: `expected/exit-code.txt`, `expected/stdout-contains.yaml`, `expected/consumer/`
