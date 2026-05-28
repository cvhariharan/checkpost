import { redirect } from '@sveltejs/kit'

export function load(): never {
  throw redirect(307, '/inventory')
}
