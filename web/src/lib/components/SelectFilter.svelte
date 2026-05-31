<script lang="ts">
  import SelectDropdown from './SelectDropdown.svelte'

  type Option = string | { value: string; label: string }

  let {
    options = [],
    value = $bindable(''),
    label = 'Filter',
    allLabel = `All`
  }: {
    options?: Option[]
    value?: string
    label?: string
    allLabel?: string
  } = $props()

  const dropdownOptions = $derived([
    { value: '', label: allLabel },
    ...options.map((option) =>
      typeof option === 'string' ? { value: option, label: option } : option
    )
  ])
</script>

<SelectDropdown {label} options={dropdownOptions} bind:value />
