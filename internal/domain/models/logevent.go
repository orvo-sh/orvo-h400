package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type LogLevel string

const (
	LogLevel_Debug LogLevel = "debug"
	LogLevel_Info  LogLevel = "info"
	LogLevel_Warn  LogLevel = "warn"
	LogLevel_Error LogLevel = "error"
	LogLevel_Fatal LogLevel = "fatal"
)

func (l *LogLevel) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	s = strings.ToLower(s)
	switch LogLevel(s) {
	case LogLevel_Debug, LogLevel_Info, LogLevel_Warn, LogLevel_Error, LogLevel_Fatal:
		*l = LogLevel(s)
		return nil
	default:
		return fmt.Errorf("invalid log level: %q", s)
	}
}

type LogEvent struct {
	ID             string         `json:"id"`
	Timestamp      time.Time      `json:"timestamp"`
	Level          LogLevel       `json:"level"`
	Message        string         `json:"message"`
	Service        string         `json:"service"`
	Environment    string         `json:"environment"`
	OrganizationID string         `json:"organization_id"`
	TraceID        string         `json:"trace_id"`
	SpanID         string         `json:"span_id"`
	ParentID       string         `json:"parent_id"`
	Attributes     map[string]any `json:"attributes"`
}
