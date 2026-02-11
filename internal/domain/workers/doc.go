// Package workers provides background worker goroutines for the orvo application.
// Workers live alongside services in the domain layer and handle periodic
// computations that cannot be expressed as ClickHouse materialized views
// (e.g. Apdex scores, health scores, error budgets).
package workers
