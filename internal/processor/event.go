package processor

// Event models the subset of a Sentry event required by the first vertical slice.
type Event struct {
	EventID     string           `json:"event_id"`
	Platform    string           `json:"platform"`
	Environment string           `json:"environment"`
	Release     string           `json:"release"`
	Level       string           `json:"level"`
	Culprit     string           `json:"culprit"`
	Transaction string           `json:"transaction"`
	Message     string           `json:"message"`
	Fingerprint []string         `json:"fingerprint"`
	LogEntry    *LogEntry        `json:"logentry"`
	Exception   *ExceptionValues `json:"exception"`
}

// LogEntry captures Sentry's logentry payload.
type LogEntry struct {
	Message   string `json:"message"`
	Formatted string `json:"formatted"`
}

// ExceptionValues captures Sentry's exception.values array.
type ExceptionValues struct {
	Values []ExceptionValue `json:"values"`
}

// ExceptionValue contains the main exception metadata used for grouping.
type ExceptionValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
