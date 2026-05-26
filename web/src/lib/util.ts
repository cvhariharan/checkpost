import type { Machine } from './api'

export function formatTimestamp(value?: string | null): string {
  if (!value) return ''
  try {
    return new Date(value).toLocaleString()
  } catch {
    return value
  }
}

export function isOnline(machine: Machine | null | undefined, windowMs = 5 * 60 * 1000): boolean {
  const ts = machine?.last_seen_at || machine?.enrolled_at
  if (!ts) return false
  const seenAt = new Date(ts)
  if (Number.isNaN(seenAt.getTime())) return false
  return Date.now() - seenAt.getTime() < windowMs
}

export function machineHostname(machine: Machine | null | undefined): string {
  return machine?.hostname || machine?.Hostname || machine?.host_identifier || 'Unknown'
}

export function machineOS(machine: Machine | null | undefined): string {
  return [machine?.os_name, machine?.os_version].filter(Boolean).join(' ') || machine?.platform || 'Unknown'
}

type ToastOptions = {
  variant?: 'success' | 'danger' | 'warning'
  placement?: string
  duration?: number
}

export function toast(message: string, title?: string, options?: ToastOptions): void {
  if (typeof window === 'undefined') return
  const ot = (window as any).ot
  ot?.toast?.(message, title, options)
}

export function formatBytes(bytes: number | undefined | null): string {
  if (bytes === undefined || bytes === null || !Number.isFinite(bytes) || bytes <= 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let n = bytes
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n.toFixed(n < 10 ? 2 : 1)} ${units[i]}`
}

export function formatGB(gb: number | undefined | null): string {
  if (gb === undefined || gb === null || !Number.isFinite(gb) || gb <= 0) return '—'
  return gb >= 1024 ? `${(gb / 1024).toFixed(2)} TB` : `${gb.toFixed(gb < 10 ? 2 : 1)} GB`
}

export function formatUptime(seconds: number | undefined | null): string {
  if (!seconds || !Number.isFinite(seconds) || seconds <= 0) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const parts: string[] = []
  if (d) parts.push(`${d}d`)
  if (h) parts.push(`${h}h`)
  if (m || parts.length === 0) parts.push(`${m}m`)
  return parts.join(' ')
}
