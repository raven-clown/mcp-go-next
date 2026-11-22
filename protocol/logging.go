package protocol

// LoggingLevel is a syslog severity (RFC 5424). Deprecated in 2026-07-28 and still supported.
type LoggingLevel string

const (
	LogDebug     LoggingLevel = "debug"
	LogInfo      LoggingLevel = "info"
	LogNotice    LoggingLevel = "notice"
	LogWarning   LoggingLevel = "warning"
	LogError     LoggingLevel = "error"
	LogCritical  LoggingLevel = "critical"
	LogAlert     LoggingLevel = "alert"
	LogEmergency LoggingLevel = "emergency"
)

var levelRank = map[LoggingLevel]int{
	LogDebug: 0, LogInfo: 1, LogNotice: 2, LogWarning: 3,
	LogError: 4, LogCritical: 5, LogAlert: 6, LogEmergency: 7,
}

// Rank orders levels from debug (0) to emergency (7). It is -1 for an empty or unknown level.
func (l LoggingLevel) Rank() int {
	if r, ok := levelRank[l]; ok {
		return r
	}
	return -1
}
