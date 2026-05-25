<script lang="ts">
  export let options: string[] = []
  export let value = ''
  export let label = 'Filter'
  export let allLabel = `All`
</script>

<label data-field>
  <span>{label}</span>
  <select bind:value>
    <option value="">{allLabel}</option>
    {#each options as option}
      <option value={option}>{option}</option>
    {/each}
  </select>
</label>
