import { formatBytes, formatGB, formatUptime } from './util'

// Closed vocabulary for `x-*` schema extras. Adding a value here is a
// vocabulary change — keep this union and specs/device-metrics.md §4 in
// lockstep with MetricRenderer.svelte. Do not widen to `string`.
export type XDisplay = 'table' | 'pills'
export type XUnit = 'bytes' | 'gigabytes' | 'duration-seconds' | 'percent'
export type XProgress = 'inverse'

export type JSONSchema = {
  type?: string | string[]
  title?: string
  description?: string
  format?: string
  properties?: Record<string, JSONSchema>
  required?: string[]
  items?: JSONSchema
  'x-display'?: XDisplay
  'x-unit'?: XUnit
  'x-primary'?: boolean
  'x-progress'?: XProgress
}

export type MetricSchemas = {
  schemas: Record<string, JSONSchema>
  kinds: string[]
}

export type MetricEntry = {
  kind: string
  value: unknown
  collected_at?: string
  updated_at?: string
}

export function formatScalar(schema: JSONSchema | undefined, value: unknown): string {
  if (value === undefined || value === null || value === '') return '—'
  if (typeof value === 'boolean') return value ? '✓' : '✗'
  if (typeof value === 'number' && schema) {
    switch (schema['x-unit']) {
      case 'bytes':
        return formatBytes(value)
      case 'gigabytes':
        return formatGB(value)
      case 'duration-seconds':
        return formatUptime(value)
      case 'percent':
        return `${Math.max(0, Math.min(100, value)).toFixed(1)}%`
    }
    return Number.isInteger(value) ? String(value) : value.toFixed(2)
  }
  return String(value)
}

export function progressFor(schema: JSONSchema | undefined, value: unknown): number | null {
  if (schema?.['x-progress'] !== 'inverse' || typeof value !== 'number') return null
  return Math.max(0, Math.min(100, 100 - value))
}

export function usageVariant(usedPct: number): 'success' | 'warning' | 'danger' {
  if (usedPct >= 90) return 'danger'
  if (usedPct >= 75) return 'warning'
  return 'success'
}

export type RootShape =
  | { kind: 'table'; arrayProp: string; itemSchema: JSONSchema }
  | { kind: 'card'; properties: [string, JSONSchema][] }

// Decide how to render the root object: a single-array-of-objects schema
// renders as a table; everything else renders as a summary card.
export function rootShape(schema: JSONSchema | undefined): RootShape {
  if (!schema || !schema.properties) return { kind: 'card', properties: [] }
  const props = Object.entries(schema.properties)
  if (props.length === 1) {
    const [name, sub] = props[0]
    if (sub.type === 'array' && sub.items?.type === 'object') {
      return { kind: 'table', arrayProp: name, itemSchema: sub.items }
    }
  }
  return { kind: 'card', properties: props }
}

export function primaryProperty(properties: [string, JSONSchema][]): string | null {
  const primary = properties.find(([, s]) => s['x-primary'])
  return primary ? primary[0] : null
}

export function isPills(schema: JSONSchema | undefined): boolean {
  return !!schema && schema['x-display'] === 'pills'
}
