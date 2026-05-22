<script>
  export let currentPage = 1
  export let pageCount = 1
  export let disabled = false
  export let label = 'Pagination'
  export let onPageChange = () => {}

  $: pages = Array.from({ length: Math.max(1, pageCount) }, (_, index) => index + 1)

  function goToPage(event, page) {
    event.preventDefault()
    if (disabled || page < 1 || page > pageCount || page === currentPage) {
      return
    }
    onPageChange(page)
  }

  function disabledAttrs(isDisabled) {
    return isDisabled ? { 'aria-disabled': 'true', tabindex: '-1' } : {}
  }
</script>

<nav aria-label={label}>
  <menu class="buttons">
    <li>
      <a
        href="#pagination"
        class="button outline small"
        {...disabledAttrs(disabled || currentPage === 1)}
        onclick={(event) => goToPage(event, currentPage - 1)}
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
          {...disabledAttrs(disabled)}
          onclick={(event) => goToPage(event, page)}
        >
          {page}
        </a>
      </li>
    {/each}
    <li>
      <a
        href="#pagination"
        class="button outline small"
        {...disabledAttrs(disabled || currentPage === pageCount)}
        onclick={(event) => goToPage(event, currentPage + 1)}
      >
        Next &rarr;
      </a>
    </li>
  </menu>
</nav>

<style>
  a[aria-disabled='true'] {
    opacity: .5;
    cursor: not-allowed;
    pointer-events: none;
  }
</style>
