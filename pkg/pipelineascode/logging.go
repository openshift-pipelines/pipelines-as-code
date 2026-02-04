package pipelineascode

// debugf logs only when a logger is available.
// This avoids nil pointer panics in tests that don't wire a logger.
func (p *PacRun) debugf(format string, args ...any) {
	if p == nil || p.logger == nil {
		return
	}
	p.logger.Debugf(format, args...)
}
