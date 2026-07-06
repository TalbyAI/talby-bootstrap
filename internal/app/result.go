package app

type ExitCode int

const (
	ExitSuccess ExitCode = iota
	ExitOperationalOrValidationError
	ExitUserActionConflict
	ExitTrustOrPolicyDenial
)

type Result struct {
	Code     ExitCode       `json:"code"`
	Message  string         `json:"message"`
	Details  map[string]any `json:"details,omitempty"`
	Warnings []string       `json:"warnings,omitempty"`
}

func Success(message string) Result {
	return Result{Code: ExitSuccess, Message: message}
}
