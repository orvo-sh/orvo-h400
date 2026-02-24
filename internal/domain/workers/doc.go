// Package workers provides background worker goroutines for the orvo application.
// Workers live alongside services in the domain layer and handle partition
// precreation, retention cleanup, archive export/retention, and restore jobs.
package workers
