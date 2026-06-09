<script lang="ts">
  import { page } from '$app/state'

  let {
    currentPage = 1,
    pageCount = 1,
    disabled = false,
    label = 'Pagination',
    param = '',
    onPageChange = () => {}
  }: {
    currentPage?: number
    pageCount?: number
    disabled?: boolean
    label?: string
    param?: string
    onPageChange?: (page: number) => void
  } = $props()

  const pages = $derived(Array.from({ length: Math.max(1, pageCount) }, (_, index) => index + 1))

  function hrefFor(target: number): string {
    const params = new URLSearchParams(page.url.search)
    if (target <= 1) params.delete(param)
    else params.set(param, String(target))
    const qs = params.toString()
    return qs ? `${page.url.pathname}?${qs}` : page.url.pathname
  }

  function goToPage(event: MouseEvent, target: number) {
    const noop = disabled || target < 1 || target > pageCount || target === currentPage
    if (param) {
      // Anchor-link mode: let SvelteKit follow the href; only block no-op clicks.
      if (noop) event.preventDefault()
      return
    }
    // Callback mode (dialogs).
    event.preventDefault()
    if (noop) return
    onPageChange(target)
  }
</script>

<nav aria-label={label}>
  <menu class="buttons">
    <li>
      <a
        href={param ? hrefFor(currentPage - 1) : '#pagination'}
        class="button outline small"
        aria-disabled={disabled || currentPage === 1 ? 'true' : undefined}
        tabindex={disabled || currentPage === 1 ? -1 : 0}
        data-sveltekit-noscroll
        onclick={(e) => goToPage(e, currentPage - 1)}
      >
        &larr; Previous
      </a>
    </li>
    {#each pages as n}
      <li>
        <a
          href={param ? hrefFor(n) : '#pagination'}
          class={currentPage === n ? 'button small' : 'button outline small'}
          aria-current={currentPage === n ? 'page' : undefined}
          aria-disabled={disabled ? 'true' : undefined}
          tabindex={disabled ? -1 : 0}
          data-sveltekit-noscroll
          onclick={(e) => goToPage(e, n)}
        >
          {n}
        </a>
      </li>
    {/each}
    <li>
      <a
        href={param ? hrefFor(currentPage + 1) : '#pagination'}
        class="button outline small"
        aria-disabled={disabled || currentPage === pageCount ? 'true' : undefined}
        tabindex={disabled || currentPage === pageCount ? -1 : 0}
        data-sveltekit-noscroll
        onclick={(e) => goToPage(e, currentPage + 1)}
      >
        Next &rarr;
      </a>
    </li>
  </menu>
</nav>

<style>
  a[aria-disabled='true'] {
    opacity: 0.5;
    cursor: not-allowed;
    pointer-events: none;
  }
</style>
