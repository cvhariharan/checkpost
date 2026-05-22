<script>
  export let value = ''
  export let placeholder = 'Search...'
  export let onSearch = () => {}

  function handleInput() {
    onSearch(value)
  }
</script>

<label>
  <span>Search</span>
  <input type="search" bind:value oninput={handleInput} placeholder={placeholder} />
</label>
