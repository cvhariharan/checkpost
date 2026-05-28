<script lang="ts">
  import SelectDropdown from './SelectDropdown.svelte'

  type Option = string | { value: string; label: string }

  export let options: Option[] = []
  export let value = ''
  export let label = 'Filter'
  export let allLabel = `All`

  $: dropdownOptions = [
    { value: '', label: allLabel },
    ...options.map((option) =>
      typeof option === 'string' ? { value: option, label: option } : option
    )
  ]
</script>

<SelectDropdown {label} options={dropdownOptions} bind:value />
