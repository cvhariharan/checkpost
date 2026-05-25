<script lang="ts">
  export let currentPage = 1
  export let pageCount = 1
  export let disabled = false
  export let label = 'Pagination'
  export let onPageChange: (page: number) => void = () => {}

  $: pages = Array.from({ length: Math.max(1, pageCount) }, (_, index) => index + 1)

  function goToPage(event: MouseEvent, page: number) {
    event.preventDefault()
    if (disabled || page < 1 || page > pageCount || page === currentPage) return
    onPageChange(page)
  }
</script>

<nav aria-label={label}>
  <menu class="buttons">
    <li>
      <a
        href="#pagination"
        class="button outline small"
        aria-disabled={disabled || currentPage === 1 ? 'true' : undefined}
        tabindex={disabled || currentPage === 1 ? -1 : 0}
        onclick={(e) => goToPage(e, currentPage - 1)}
      >
        &larr; Previous
      </a>
    </li>
    {#each pages as page}
      <li>
        <a
          href="#pagination"
          class={currentPage === page ? 'button small' : 'button outline small'}
          aria-current={currentPage === page ? 'page' : undefined}
          aria-disabled={disabled ? 'true' : undefined}
          tabindex={disabled ? -1 : 0}
          onclick={(e) => goToPage(e, page)}
        >
          {page}
        </a>
      </li>
    {/each}
    <li>
      <a
        href="#pagination"
        class="button outline small"
        aria-disabled={disabled || currentPage === pageCount ? 'true' : undefined}
        tabindex={disabled || currentPage === pageCount ? -1 : 0}
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
