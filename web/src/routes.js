import { writable } from 'svelte/store'
import MachinesPage from './pages/MachinesPage.svelte'
import MachineQueryPage from './pages/MachineQueryPage.svelte'
import QueriesPage from './pages/QueriesPage.svelte'
import PacksPage from './pages/PacksPage.svelte'
import SchedulesPage from './pages/SchedulesPage.svelte'
import ErrorPage from './pages/ErrorPage.svelte'

const routes = [
  { path: /^\/machines\/?$/, component: MachinesPage, section: 'machines' },
  {
    path: /^\/machines\/([^/]+)\/query\/?$/,
    component: MachineQueryPage,
    section: 'machines',
    params: ['id']
  },
  { path: /^\/queries\/?$/, component: QueriesPage, section: 'queries' },
  { path: /^\/packs\/?$/, component: PacksPage, section: 'packs' },
  { path: /^\/schedules\/?$/, component: SchedulesPage, section: 'schedules' },
  { path: /^\/error\/?$/, component: ErrorPage, section: 'error' }
]

function currentQuery() {
  return Object.fromEntries(new URLSearchParams(window.location.search))
}

function matchPath(pathname) {
  if (pathname === '/') {
    return { redirect: '/machines' }
  }

  for (const routeDef of routes) {
    const match = pathname.match(routeDef.path)
    if (match) {
      const params = {}
      for (const [index, name] of (routeDef.params || []).entries()) {
        params[name] = decodeURIComponent(match[index + 1])
      }

      return {
        component: routeDef.component,
        params,
        path: pathname,
        query: currentQuery(),
        section: routeDef.section
      }
    }
  }

  return {
    component: ErrorPage,
    params: { code: '404', message: 'Page not found.' },
    path: pathname,
    query: currentQuery(),
    section: 'error'
  }
}

function syncRoute() {
  const matched = matchPath(window.location.pathname)
  if (matched.redirect) {
    navigate(matched.redirect, { replace: true })
    return
  }

  route.set(matched)
}

export const route = writable({
  component: MachinesPage,
  params: {},
  path: '/machines',
  query: {},
  section: 'machines'
})

export function startRouter() {
  syncRoute()
  window.addEventListener('popstate', syncRoute)
  return () => window.removeEventListener('popstate', syncRoute)
}

export function navigate(to, { replace = false } = {}) {
  const url = new URL(to, window.location.origin)
  const current = `${window.location.pathname}${window.location.search}`
  const next = `${url.pathname}${url.search}`

  if (current !== next) {
    window.history[replace ? 'replaceState' : 'pushState']({}, '', next)
  }

  syncRoute()
}

export function link(node, href) {
  let target = href

  function handleClick(event) {
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.altKey ||
      event.ctrlKey ||
      event.shiftKey
    ) {
      return
    }

    const url = new URL(target, window.location.origin)
    if (url.origin !== window.location.origin) {
      return
    }

    event.preventDefault()
    navigate(`${url.pathname}${url.search}`)
  }

  node.addEventListener('click', handleClick)

  return {
    update(nextHref) {
      target = nextHref
    },
    destroy() {
      node.removeEventListener('click', handleClick)
    }
  }
}
