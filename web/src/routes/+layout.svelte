<script lang="ts">
  import '@knadh/oat/oat.min.css'
  import '$lib/styles/theme.css'
  import '@knadh/oat/oat.min.js'
  import { onMount, type Component, type Snippet } from 'svelte'
  import { page } from '$app/state'
  import { goto } from '$app/navigation'
  import Logo from '$lib/components/Logo.svelte'
  import ThemeToggle from '$lib/components/ThemeToggle.svelte'
  import { logout as apiLogout, fetchInfo, type BuildInfo, type Me } from '$lib/api'
  import { canFrom, setMe } from '$lib/auth'
  import Server from '@lucide/svelte/icons/server'
  import Boxes from '@lucide/svelte/icons/boxes'
  import ShieldCheck from '@lucide/svelte/icons/shield-check'
  import CalendarClock from '@lucide/svelte/icons/calendar-clock'
  import ScanSearch from '@lucide/svelte/icons/scan-search'
  import Bell from '@lucide/svelte/icons/bell'
  import User from '@lucide/svelte/icons/user'
  import Users from '@lucide/svelte/icons/users'
  import KeyRound from '@lucide/svelte/icons/key-round'
  import PanelLeft from '@lucide/svelte/icons/panel-left'
  import ChevronsLeft from '@lucide/svelte/icons/chevrons-left'
  import LogOut from '@lucide/svelte/icons/log-out'

  let { data, children }: { data: { me: Me | null }; children: Snippet } = $props()

  $effect(() => {
    setMe(data?.me ?? null)
  })

  const user = $derived(data?.me?.user ?? null)

  let info = $state<BuildInfo | null>(null)
  onMount(() => {
    fetchInfo()
      .then((i) => (info = i))
      .catch(() => {})
  })

  type NavItem = {
    href: string
    label: string
    section: string
    resource: string
    action: string
    icon: Component<any>
  }

  const navItems: NavItem[] = [
    { href: '/inventory', label: 'Inventory', section: 'inventory', resource: 'machine', action: 'view', icon: Server },
    { href: '/groups', label: 'Groups', section: 'groups', resource: 'machine_group', action: 'view', icon: Boxes },
    { href: '/policies', label: 'Policies', section: 'policies', resource: 'policy', action: 'view', icon: ShieldCheck },
    { href: '/schedules', label: 'Schedules', section: 'schedules', resource: 'schedule', action: 'view', icon: CalendarClock },
    { href: '/yara', label: 'YARA', section: 'yara', resource: 'yara_source', action: 'view', icon: ScanSearch },
    { href: '/alerts', label: 'Alerts', section: 'alerts', resource: 'alert_rule', action: 'view', icon: Bell }
  ]

  const adminItems: NavItem[] = [
    { href: '/admin/users', label: 'Users', section: 'users', resource: 'user', action: 'view', icon: User },
    { href: '/admin/user-groups', label: 'User Groups', section: 'user-groups', resource: 'user_group', action: 'view', icon: Users }
  ]

  const pathname = $derived(page.url.pathname)
  const rootSection = $derived(pathname.split('/')[1] || '')
  const section = $derived(
    rootSection === 'machines'
      ? 'inventory'
      : rootSection === 'admin' || rootSection === 'settings'
        ? pathname.split('/')[2] || ''
        : rootSection
  )
  const isLoginRoute = $derived(pathname === '/login')
  const visibleNav = $derived(navItems.filter((item) => {
    return canFrom(data?.me ?? null, item.resource, item.action)
  }))
  const visibleAdminNav = $derived(adminItems.filter((item) => {
    return canFrom(data?.me ?? null, item.resource, item.action)
  }))
  const admin = $derived(visibleAdminNav.length > 0)

  async function handleLogout() {
    try {
      await apiLogout()
    } catch {
      // ignore; clear locally regardless
    }
    setMe(null)
    goto('/login')
  }
</script>

{#if isLoginRoute}
  {@render children()}
{:else}
  <div data-sidebar-layout="always">
    <nav data-topnav aria-label="Application">
      <button
        type="button"
        class="icon ghost small topnav-toggle"
        data-sidebar-toggle
        aria-label="Show navigation"
      >
        <PanelLeft size={16} aria-hidden="true" />
      </button>
      <a href="/inventory" class="unstyled topnav-brand">
        <Logo size="sm" />
      </a>
    </nav>

    <aside data-sidebar>
      <nav aria-label="Primary navigation">
        <ul>
          {#each visibleNav as item}
            {@const Icon = item.icon}
            <li>
              <a
                href={item.href}
                aria-current={section === item.section ? 'page' : undefined}
                data-tooltip={item.label}
                data-tooltip-placement="right"
              >
                <Icon size={18} aria-hidden="true" />
                <span class="nav-label">{item.label}</span>
              </a>
            </li>
          {/each}
        </ul>

        {#if admin}
          <p class="admin-heading text-light">Admin</p>
          <ul>
            {#each visibleAdminNav as item}
              {@const Icon = item.icon}
              <li>
                <a
                  href={item.href}
                  aria-current={section === item.section ? 'page' : undefined}
                  data-tooltip={item.label}
                  data-tooltip-placement="right"
                >
                  <Icon size={18} aria-hidden="true" />
                  <span class="nav-label">{item.label}</span>
                </a>
              </li>
            {/each}
          </ul>
        {/if}

        {#if user}
          <p class="admin-heading text-light">Settings</p>
          <ul>
            <li>
              <a
                href="/settings/api-tokens"
                aria-current={section === 'api-tokens' ? 'page' : undefined}
                data-tooltip="API Tokens"
                data-tooltip-placement="right"
              >
                <KeyRound size={18} aria-hidden="true" />
                <span class="nav-label">API Tokens</span>
              </a>
            </li>
          </ul>
        {/if}
      </nav>

      {#if info}
        <span class="app-version text-light">{info.version}-{info.commit}</span>
      {/if}

      <footer class="sidebar-footer vstack gap-2">
        {#if user}
          <div class="user-block">
            <strong class="user-name">{user.name || user.username}</strong>
            <span class="text-light user-email">{user.email || user.username}</span>
          </div>
          <button
            type="button"
            class="outline small w-full gap-1 logout-button"
            onclick={handleLogout}
            data-tooltip="Logout"
            data-tooltip-placement="right"
          >
            <LogOut size={16} aria-hidden="true" />
            <span class="logout-label">Logout</span>
          </button>
        {/if}
        <div class="hstack justify-between footer-controls">
          <ThemeToggle />
          <button
            type="button"
            class="icon ghost small sidebar-toggle"
            data-sidebar-toggle
            aria-label="Toggle navigation"
            data-tooltip="Toggle navigation"
            data-tooltip-placement="right"
          >
            <ChevronsLeft size={16} aria-hidden="true" />
          </button>
        </div>
      </footer>
    </aside>

    <main>
      <div class="container mb-6">
        {@render children()}
      </div>
    </main>
  </div>
{/if}

<style>
  .topnav-brand {
    display: inline-flex;
    align-items: center;
    color: var(--foreground);
  }
  :global([data-sidebar] > footer.sidebar-footer) {
    border-top: 1px solid var(--border);
    padding-top: var(--space-2);
  }
  .admin-heading {
    margin: var(--space-3) 0 var(--space-1);
    font-size: var(--font-sm, 0.85rem);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .user-block {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .user-name,
  .user-email {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .w-full {
    width: 100%;
  }
  .app-version {
    display: block;
    font-size: var(--font-sm, 0.8rem);
    padding-inline: var(--space-3);
    padding-block-end: var(--space-2);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .topnav-toggle {
    display: none;
  }
  @media (max-width: 768px) {
    :global([data-sidebar-layout]:not([data-sidebar-open])) .topnav-toggle {
      display: inline-flex;
    }
  }

  /* Sidebar links: vertically center icon + label */
  :global([data-sidebar] nav a) {
    align-items: center;
  }

  :global([data-sidebar] > nav) {
    overflow-x: hidden;
  }

  /* oat draws a border box on [data-sidebar-toggle]; keep the footer toggle a clean ghost icon */
  .sidebar-toggle {
    border-color: transparent;
    border-radius: var(--radius-medium);
  }

  /* Tooltips on sidebar links/logout only appear in collapsed rail mode */
  :global([data-sidebar] nav a[data-tooltip]:hover::after),
  :global([data-sidebar] nav a[data-tooltip]:hover::before),
  :global([data-sidebar] .logout-button[data-tooltip]:hover::after),
  :global([data-sidebar] .logout-button[data-tooltip]:hover::before) {
    opacity: 0;
    visibility: hidden;
  }

  /* Collapsed sidebar becomes a narrow icon rail (desktop) */
  @media (min-width: 769px) {
    :global([data-sidebar-layout="always"][data-sidebar-open]) {
      grid-template-columns: 3.5rem 1fr;
    }
    :global([data-sidebar-layout="always"][data-sidebar-open] > aside[data-sidebar]) {
      overflow: visible;
      min-width: 0;
      transform: none;
      opacity: 1;
      visibility: visible;
      border-inline-end: 1px solid var(--border);
    }
    /* let tooltips escape the rail instead of being clipped */
    :global([data-sidebar-open] [data-sidebar] > nav) {
      overflow: visible;
    }
    /* hide all text, keep icons */
    :global([data-sidebar-open] .nav-label),
    :global([data-sidebar-open] .admin-heading),
    :global([data-sidebar-open] .logout-label),
    :global([data-sidebar-open] .app-version),
    :global([data-sidebar-open] .user-block) {
      display: none;
    }
    /* center the icons within the rail */
    :global([data-sidebar-open] [data-sidebar] nav a),
    :global([data-sidebar-open] .logout-button) {
      justify-content: center;
      padding-inline: 0;
    }
    :global([data-sidebar-open] .footer-controls) {
      flex-direction: column;
      gap: var(--space-1);
    }
    /* the footer toggle is the only expand control while collapsed — point it right */
    :global([data-sidebar-open] .sidebar-toggle svg) {
      transform: rotate(180deg);
    }
    /* re-enable link tooltips while collapsed */
    :global([data-sidebar-open] [data-sidebar] nav a[data-tooltip]:hover::after),
    :global([data-sidebar-open] [data-sidebar] nav a[data-tooltip]:hover::before),
    :global([data-sidebar-open] [data-sidebar] .logout-button[data-tooltip]:hover::after),
    :global([data-sidebar-open] [data-sidebar] .logout-button[data-tooltip]:hover::before) {
      opacity: 1;
      visibility: visible;
    }
  }

  :global(dialog[open]:has(> form > div.vstack)) {
    display: flex;
    flex-direction: column;
  }
  :global(dialog[open]:has(> form > div.vstack) > form) {
    display: flex;
    flex-direction: column;
    min-height: 0;
    max-height: 100%;
  }
  :global(dialog[open] > form > div.vstack) {
    flex: 1 1 auto;
    min-height: 0;
  }

  :global(ot-dropdown [role="menuitem"]) {
    background-color: transparent;
  }
  :global(ot-dropdown [role="menuitem"]:hover),
  :global(ot-dropdown [role="menuitem"]:focus-visible) {
    background-color: var(--accent);
  }

  :global(.sr-only) {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  :global(.cell-link) {
    display: inline;
    padding: 0;
    margin: 0;
    background: none;
    border: none;
    font: inherit;
    font-weight: var(--font-semibold);
    color: var(--primary);
    text-align: start;
    cursor: pointer;
  }
  :global(.cell-link:hover) {
    text-decoration: underline;
  }
</style>
