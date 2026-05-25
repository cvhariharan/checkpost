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
