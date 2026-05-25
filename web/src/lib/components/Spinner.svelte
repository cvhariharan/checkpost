<script lang="ts">
  export let size: 'small' | 'large' | '' = ''
  export let label = 'Loading'
</script>

<div aria-busy="true" data-spinner={size || undefined} aria-label={label}></div>
