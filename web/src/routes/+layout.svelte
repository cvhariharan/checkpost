<script lang="ts">
  import '@knadh/oat/oat.min.css'
  import '$lib/styles/theme.css'
  import '@knadh/oat/oat.min.js'
  import { page } from '$app/stores'

  const navItems: { href: string; label: string; section: string }[] = [
    { href: '/machines', label: 'Machines', section: 'machines' },
    { href: '/groups', label: 'Groups', section: 'groups' },
    { href: '/policies', label: 'Policies', section: 'policies' },
    { href: '/schedules', label: 'Schedules', section: 'schedules' }
  ]

  $: section = $page.url.pathname.split('/')[1] || ''
</script>

<div data-sidebar-layout="always">
  <nav data-topnav aria-label="Application">
    <button type="button" data-sidebar-toggle aria-label="Toggle navigation">
      <span aria-hidden="true">☰</span>
    </button>
    <a href="/machines" class="unstyled"><strong>Watcher</strong></a>
  </nav>

  <aside data-sidebar>
    <nav aria-label="Primary navigation">
      <ul>
        {#each navItems as item}
          <li>
            <a href={item.href} aria-current={section === item.section ? 'page' : undefined}>
              {item.label}
            </a>
          </li>
        {/each}
      </ul>
    </nav>
  </aside>

  <main>
    <div class="container mb-6">
      <slot />
    </div>
  </main>
</div>

<style>
  :global(dialog:has(> form > div.vstack)) {
    display: flex;
    flex-direction: column;
  }
  :global(dialog:has(> form > div.vstack) > form) {
    display: flex;
    flex-direction: column;
    min-height: 0;
    max-height: 100%;
  }
  :global(dialog > form > div.vstack) {
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
</style>
