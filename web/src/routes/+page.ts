import { redirect } from '@sveltejs/kit'
import { canFrom } from '$lib/auth'

export async function load({ parent }): Promise<never> {
  const { me } = await parent()
  if (canFrom(me ?? null, 'dashboard', 'view')) {
    throw redirect(307, '/dashboard')
  }
  throw redirect(307, '/inventory')
}
