<script lang="ts">
  import SelectDropdown from './SelectDropdown.svelte'

  export let options: string[] = []
  export let value = ''
  export let label = 'Filter'
  export let allLabel = `All`

  $: dropdownOptions = [
    { value: '', label: allLabel },
    ...options.map((option) => ({ value: option, label: option }))
  ]
</script>

<SelectDropdown {label} options={dropdownOptions} bind:value />
