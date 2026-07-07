# Ambiguous install target rejected

Purpose: prove that an ambiguous or invalid explicit target form is rejected.

Represented command: `tbboot install file:local-example-source --artifact ""`

Kind: `atomic-case`

Polarity: `negative`

Normative outputs: `expected/exit-code.txt`, `expected/stdout-contains.yaml`, `expected/consumer/`
