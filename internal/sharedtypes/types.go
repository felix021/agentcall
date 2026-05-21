package sharedtypes

type CallbackPayload struct {
	Token       string            `json:"token"`
	Status      string            `json:"status"`
	ContentType string            `json:"content_type"`
	Content     string            `json:"content"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type ResultEnvelope struct {
	RunID       string `json:"run_id"`
	State       string `json:"state"`
	Status      string `json:"status"`
	ExitCode    int    `json:"exit_code"`
	ContentType string `json:"content_type"`
	Content     string `json:"content"`
	Error       string `json:"error"`
}
