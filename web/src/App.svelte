<script>
  import { onMount } from 'svelte'
  import { link, route, startRouter } from './routes'

  const navItems = [
    { href: '/machines', label: 'Machines', section: 'machines' },
    { href: '/queries', label: 'Queries', section: 'queries' },
    { href: '/packs', label: 'Packs', section: 'packs' },
    { href: '/schedules', label: 'Schedules', section: 'schedules' }
  ]

  onMount(startRouter)
</script>

<div data-sidebar-layout="always">
  <nav data-topnav aria-label="Application">
    <button type="button" data-sidebar-toggle aria-label="Toggle navigation">Menu</button>
    <a href="/machines" use:link={'/machines'} class="unstyled">
      <strong>Watcher</strong>
    </a>
  </nav>

  <aside data-sidebar>
    <header>
      <strong>Watcher</strong>
      <small class="text-light">osquery control</small>
    </header>

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
