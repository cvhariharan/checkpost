// Canonical policy severity levels, most-severe first. Mirrors the backend
// models.PolicySeverities and the DB check constraint — keep in sync.

export const severityOptions: { value: string; label: string }[] = [
  { value: 'critical', label: 'Critical' },
  { value: 'high', label: 'High' },
  { value: 'medium', label: 'Medium' },
  { value: 'low', label: 'Low' },
  { value: 'info', label: 'Info' }
]

// Maps a severity to its badge variant (oat data-variant).
export const severityVariant: Record<string, string> = {
  critical: 'danger',
  high: 'danger',
  medium: 'warning',
  low: 'info',
  info: 'info'
}
