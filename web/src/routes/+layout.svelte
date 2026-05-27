<script lang="ts">
  import '@knadh/oat/oat.min.css'
  import '$lib/styles/theme.css'
  import '@knadh/oat/oat.min.js'
  import { page } from '$app/stores'
  import ThemeToggle from '$lib/components/ThemeToggle.svelte'

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

    <footer>
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
    </footer>
  </aside>

  <main>
    <div class="container mb-6">
      <slot />
    </div>
  </main>
</div>

<style>
  :global([data-sidebar] > footer) {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    border-top: 1px solid var(--border);
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
