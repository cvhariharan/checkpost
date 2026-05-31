import { redirect } from '@sveltejs/kit'
import type { Me } from '$lib/api'

export const ssr = false
export const prerender = false
export const trailingSlash = 'never'

// Client-side session guard: fetch the current user; bounce to /login on 401.
export async function load({ url, fetch }): Promise<{ me: Me | null }> {
  if (url.pathname === '/login') {
    return { me: null }
  }

  const res = await fetch('/api/v1/me').catch(() => null)
  if (!res || res.status === 401) {
    const target = url.pathname + url.search
    throw redirect(307, `/login?redirect_url=${encodeURIComponent(target)}`)
  }
  if (!res.ok) {
    return { me: null }
  }

  const me = (await res.json()) as Me
  return { me }
}
