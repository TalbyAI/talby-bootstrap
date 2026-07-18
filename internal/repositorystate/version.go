package repositorystate

import "regexp"

var (
	canonicalSemVerPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	gitCommitPattern       = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

func isCanonicalSemVer(value string) bool { return canonicalSemVerPattern.MatchString(value) }
func isGitCommit(value string) bool       { return gitCommitPattern.MatchString(value) }
