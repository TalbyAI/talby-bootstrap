# Non interactive prompt required

Purpose: prove that a path requiring a prompt fails with the documented non-interactive contract.

Represented command: `tbboot install file:prompting-source --non-interactive`

Kind: `atomic-case`

Polarity: `negative`

Normative outputs: `expected/exit-code.txt`, `expected/stdout-contains.yaml`, `expected/consumer/`
