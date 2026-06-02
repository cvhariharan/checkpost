<script lang="ts">
  import '@knadh/oat/oat.min.css'
  import '$lib/styles/theme.css'
  import '@knadh/oat/oat.min.js'
  import type { Snippet } from 'svelte'
  import { page } from '$app/state'
  import { goto } from '$app/navigation'
  import ThemeToggle from '$lib/components/ThemeToggle.svelte'
  import { logout as apiLogout, type Me } from '$lib/api'
  import { canFrom, setMe } from '$lib/auth'

  let { data, children }: { data: { me: Me | null }; children: Snippet } = $props()

  $effect(() => {
    setMe(data?.me ?? null)
  })

  const user = $derived(data?.me?.user ?? null)

  type NavItem = { href: string; label: string; section: string; resource: string; action: string }

  const navItems: NavItem[] = [
    { href: '/inventory', label: 'Inventory', section: 'inventory', resource: 'machine', action: 'view' },
    { href: '/groups', label: 'Groups', section: 'groups', resource: 'machine_group', action: 'view' },
    { href: '/policies', label: 'Policies', section: 'policies', resource: 'policy', action: 'view' },
    { href: '/schedules', label: 'Schedules', section: 'schedules', resource: 'schedule', action: 'view' },
    { href: '/yara', label: 'YARA', section: 'yara', resource: 'yara_source', action: 'view' },
    { href: '/alerts', label: 'Alerts', section: 'alerts', resource: 'alert_rule', action: 'view' }
  ]

  const adminItems: NavItem[] = [
    { href: '/admin/users', label: 'Users', section: 'users', resource: 'user', action: 'view' },
    { href: '/admin/user-groups', label: 'User Groups', section: 'user-groups', resource: 'user_group', action: 'view' }
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
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.75"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <rect width="18" height="18" x="3" y="3" rx="2" />
          <path d="M9 3v18" />
        </svg>
      </button>
      <a href="/inventory" class="unstyled"><strong>Checkpost</strong></a>
    </nav>

    <aside data-sidebar>
      <nav aria-label="Primary navigation">
        <ul>
          {#each visibleNav as item}
            <li>
              <a href={item.href} aria-current={section === item.section ? 'page' : undefined}>
                {item.label}
              </a>
            </li>
          {/each}
        </ul>

        {#if admin}
          <p class="admin-heading text-light">Admin</p>
          <ul>
            {#each visibleAdminNav as item}
              <li>
                <a href={item.href} aria-current={section === item.section ? 'page' : undefined}>
                  {item.label}
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
              >
                API Tokens
              </a>
            </li>
          </ul>
        {/if}
      </nav>

      <footer class="sidebar-footer vstack gap-2">
        {#if user}
          <div class="user-block">
            <strong class="user-name">{user.name || user.username}</strong>
            <span class="text-light user-email">{user.email || user.username}</span>
          </div>
          <button type="button" class="outline small w-full" onclick={handleLogout}>Logout</button>
        {/if}
        <div class="hstack justify-between">
          <ThemeToggle />
          <button
            type="button"
            class="icon ghost small sidebar-toggle"
            data-sidebar-toggle
            aria-label="Collapse navigation"
            title="Collapse navigation"
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.75"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <rect width="18" height="18" x="3" y="3" rx="2" />
              <path d="M9 3v18" />
              <path d="m16 15-3-3 3-3" />
            </svg>
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

  .topnav-toggle {
    display: none;
  }
  @media (min-width: 769px) {
    :global([data-sidebar-layout="always"][data-sidebar-open]) .topnav-toggle {
      display: inline-flex;
    }
  }
  @media (max-width: 768px) {
    :global([data-sidebar-layout]:not([data-sidebar-open])) .topnav-toggle {
      display: inline-flex;
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
