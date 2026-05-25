<script lang="ts">
  export let value = ''
  export let placeholder = 'Search...'
  export let label = 'Search'
</script>

<label data-field>
  <span>{label}</span>
  <input type="search" bind:value {placeholder} />
</label>
