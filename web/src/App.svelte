<script>
  import { onMount } from 'svelte'
  import { link, route, startRouter } from './routes'

  const navItems = [
    { href: '/machines', label: 'Machines', section: 'machines' },
    { href: '/queries', label: 'Queries', section: 'queries' },
    { href: '/packs', label: 'Packs', section: 'packs' },
    { href: '/policies', label: 'Policies', section: 'policies' },
    { href: '/schedules', label: 'Schedules', section: 'schedules' }
  ]

  onMount(startRouter)
</script>

<div data-sidebar-layout="always">
  <nav data-topnav aria-label="Application">
    <button type="button" data-sidebar-toggle aria-label="Toggle navigation">
      <span aria-hidden="true">☰</span>
    </button>
    <a href="/machines" use:link={'/machines'} class="unstyled">
      <strong>Watcher</strong>
    </a>
  </nav>

  <aside data-sidebar>
    <nav aria-label="Primary navigation">
      <ul>
        {#each navItems as item}
          <li>
            <a
              href={item.href}
              use:link={item.href}
              aria-current={$route.section === item.section ? 'page' : undefined}
            >
              {item.label}
            </a>
          </li>
        {/each}
      </ul>
    </nav>
  </aside>

  <main>
    <div class="container mb-6">
      <svelte:component this={$route.component} params={$route.params} query={$route.query} />
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
</style>
