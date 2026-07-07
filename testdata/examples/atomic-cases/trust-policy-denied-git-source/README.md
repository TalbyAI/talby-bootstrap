# Trust policy denied git source

Purpose: prove that a `git:` source is denied until approved in the manifest trust policy.

Represented command: `tbboot install git:github.com/example/library`

Kind: `atomic-case`

Polarity: `negative`

Normative outputs: `expected/exit-code.txt`, `expected/stdout-contains.yaml`, `expected/consumer/`
