import { writable, derived, get } from 'svelte/store'
import type { Me } from '$lib/api'

// Current authenticated user + effective permissions, or null when unknown.
export const me = writable<Me | null>(null)

export function setMe(value: Me | null) {
  me.set(value)
}

export function canFrom(current: Me | null, resource: string, action: string): boolean {
  if (!current) return false
  const actions = current.permissions?.[resource]
  return Array.isArray(actions) && actions.includes(action)
}

/** can reports whether the current user holds `action` on `resource` (global scope). */
export function can(resource: string, action: string): boolean {
  return canFrom(get(me), resource, action)
}

/** hasRole reports whether the current user holds the named role. */
export function hasRole(role: string): boolean {
  const current = get(me)
  return !!current && Array.isArray(current.roles) && current.roles.includes(role)
}

/** isAdmin is true when the user can manage role bindings (admin role). */
export const isAdmin = derived(me, ($me) => !!$me && $me.roles?.includes('admin'))
