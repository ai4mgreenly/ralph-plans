package main

var validTransitions = map[string][]string{
	"draft":   {"queued", "cancelled"},
	"queued":  {"running", "cancelled"},
	"running": {"done", "stuck", "cancelled"},
	"stuck":   {"queued", "cancelled"},
}

func canTransition(from, to string) bool {
	targets, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

func isTerminal(status string) bool {
	return status == "done" || status == "cancelled"
}
