package filter

import (
	"strings"

	audit "k8s.io/apiserver/pkg/apis/audit"
)

var allowedVerbs = map[string]bool{
	"create": true,
	"update": true,
	"patch":  true,
	"delete": true,
	"get":    true,
	"list":   true,
}

func IsRealUser(username string) bool {
	if username == "" {
		return false
	}
	if username == "system:admin" || username == "kube:admin" {
		return true
	}
	if strings.HasPrefix(username, "system:") {
		return false
	}
	return true
}

func IsAllowedVerb(verb string) bool {
	return allowedVerbs[verb]
}

func ShouldKeepEvent(event *audit.Event) bool {
	if event.Stage != audit.StageResponseComplete {
		return false
	}
	if !IsRealUser(event.User.Username) {
		return false
	}
	if !IsAllowedVerb(event.Verb) {
		return false
	}
	return true
}
