<script lang="ts">
  export let size: 'small' | 'large' | '' = ''
  export let label = 'Loading'
  // `fill` centres the spinner in the page (vertical + horizontal) for full-page
  // loads. Default stays compact for inline/section use.
  export let fill = false
</script>

<div
  class="flex items-center justify-center {fill ? 'spinner-fill' : 'p-4'}"
  aria-busy="true"
  data-spinner={size || undefined}
  aria-label={label}
></div>

<style>
  /* oat has no height utility; this min-height gives the flex box room to
     centre the spinner vertically within the page content area. */
  .spinner-fill {
    min-height: 70vh;
  }
</style>
