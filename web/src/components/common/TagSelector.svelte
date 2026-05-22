<script>
  export let tags = []
  export let value = ''
  export let onChange = () => {}

  function handleChange() {
    onChange(value)
  }
</script>

<label>
  <span>Tag</span>
  <select bind:value onchange={handleChange}>
    <option value="">All Tags</option>
    {#each tags as tag}
      <option value={tag}>{tag}</option>
    {/each}
  </select>
</label>
