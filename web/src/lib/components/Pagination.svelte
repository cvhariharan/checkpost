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

  // Windowed page list: always show first/last plus a range around the current
  // page, collapsing the rest into ellipses so the control never overflows.
  const SIBLINGS = 1

  const items = $derived.by((): (number | 'ellipsis')[] => {
    const total = Math.max(1, pageCount)
    const current = Math.min(Math.max(1, currentPage), total)

    // Small enough to show every page without truncation.
    if (total <= 7) return Array.from({ length: total }, (_, index) => index + 1)

    const first = 1
    const last = total
    const start = Math.max(first + 1, current - SIBLINGS)
    const end = Math.min(last - 1, current + SIBLINGS)

    const result: (number | 'ellipsis')[] = [first]
    if (start > first + 1) result.push('ellipsis')
    for (let n = start; n <= end; n++) result.push(n)
    if (end < last - 1) result.push('ellipsis')
    result.push(last)
    return result
  })

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
    {#each items as item}
      {#if item === 'ellipsis'}
        <li>
          <span class="ellipsis" aria-hidden="true">&hellip;</span>
        </li>
      {:else}
        <li>
          <a
            href={param ? hrefFor(item) : '#pagination'}
            class={currentPage === item ? 'button small' : 'button outline small'}
            aria-current={currentPage === item ? 'page' : undefined}
            aria-disabled={disabled ? 'true' : undefined}
            tabindex={disabled ? -1 : 0}
            data-sveltekit-noscroll
            onclick={(e) => goToPage(e, item)}
          >
            {item}
          </a>
        </li>
      {/if}
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

  .ellipsis {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 2rem;
    opacity: 0.6;
    user-select: none;
  }
</style>
