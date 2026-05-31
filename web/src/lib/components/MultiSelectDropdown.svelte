<script lang="ts">
  import { tick } from 'svelte'

  type Option = { value: string; label: string }
  type RawOption = { value?: string; uuid?: string; label?: string; name?: string }

  let {
    label = '',
    options = [],
    value = $bindable<string[]>([]),
    placeholder = 'Select options',
    searchPlaceholder = 'Search...',
    emptyLabel = 'No options available',
    disabled = false
  }: {
    label?: string
    options?: RawOption[]
    value?: string[]
    placeholder?: string
    searchPlaceholder?: string
    emptyLabel?: string
    disabled?: boolean
  } = $props()

  const menuId = `multi-select-${Math.random().toString(36).slice(2, 10)}`

  let trigger = $state<HTMLButtonElement>()
  let popover = $state<HTMLDivElement>()
  let searchInput = $state<HTMLInputElement>()
  let searchTerm = $state('')

  const normalizedOptions = $derived((options || [])
    .map<Option>((option) => ({
      value: option?.value ?? option?.uuid ?? '',
      label: option?.label ?? option?.name ?? option?.value ?? option?.uuid ?? ''
    }))
    .filter((option) => option.value && option.label))
  const selectedOptions = $derived(normalizedOptions.filter((option) => value.includes(option.value)))
  const filteredOptions = $derived(normalizedOptions.filter((option) =>
    option.label.toLowerCase().includes(searchTerm.trim().toLowerCase())
  ))
  const selectionSummary = $derived(selectedOptions.length === 0 ? placeholder : `${selectedOptions.length} selected`)

  function toggleOption(optionValue: string) {
    if (value.includes(optionValue)) {
      value = value.filter((item) => item !== optionValue)
      return
    }
    value = [...value, optionValue]
  }

  function removeOption(optionValue: string) {
    value = value.filter((item) => item !== optionValue)
  }

  function clearSelection() {
    value = []
  }

  function syncMenuWidth() {
    if (!trigger || !popover) return
    const width = Math.max(trigger.getBoundingClientRect().width, 320)
    popover.style.width = `${width}px`
  }

  async function handleToggle(event: ToggleEvent) {
    if (event.newState === 'open') {
      syncMenuWidth()
      await tick()
      searchInput?.focus()
      return
    }
    searchTerm = ''
  }
</script>

<div class="multiselect vstack gap-2">
  {#if label}<span>{label}</span>{/if}

  <ot-dropdown class="multiselect-dropdown">
    <button
      bind:this={trigger}
      type="button"
      class="outline multiselect-trigger"
      popovertarget={menuId}
      {disabled}
      aria-label={label || placeholder}
    >
      <span>{selectionSummary}</span>
      <span aria-hidden="true" class="multiselect-chevron">▾</span>
    </button>

    <div bind:this={popover} id={menuId} popover class="card multiselect-menu" ontoggle={handleToggle}>
      <div class="vstack gap-2">
        <input bind:this={searchInput} bind:value={searchTerm} placeholder={searchPlaceholder} />

        <div class="multiselect-options">
          {#if filteredOptions.length === 0}
            <p class="text-light">{emptyLabel}</p>
          {:else}
            {#each filteredOptions as option}
              <button
                type="button"
                role="menuitem"
                class="small outline multiselect-option"
                data-selected={value.includes(option.value) ? 'true' : 'false'}
                onclick={() => toggleOption(option.value)}
                aria-label={`${value.includes(option.value) ? 'Remove' : 'Add'} ${option.label}`}
              >
                <span class="multiselect-option-label">{option.label}</span>
                <span class="badge" data-variant={value.includes(option.value) ? 'success' : undefined}>
                  {value.includes(option.value) ? 'Selected' : 'Add'}
                </span>
              </button>
            {/each}
          {/if}
        </div>

        {#if selectedOptions.length > 0}
          <footer class="hstack justify-between">
            <span class="text-light">{selectedOptions.length} selected</span>
            <button type="button" class="small outline" onclick={clearSelection}>Clear</button>
          </footer>
        {/if}
      </div>
    </div>
  </ot-dropdown>

  {#if selectedOptions.length > 0}
    <div class="hstack gap-2 multiselect-pills">
      {#each selectedOptions as option}
        <span class="badge outline multiselect-pill">
          <span>{option.label}</span>
          <button
            type="button"
            class="multiselect-pill-remove"
            aria-label={`Remove ${option.label}`}
            onclick={() => removeOption(option.value)}
          >
            ×
          </button>
        </span>
      {/each}
    </div>
  {/if}
</div>

<style>
  .multiselect,
  .multiselect-dropdown {
    width: 100%;
  }

  .multiselect-trigger {
    width: 100%;
    justify-content: space-between;
  }

  .multiselect-chevron {
    opacity: 0.7;
  }

  .multiselect-menu {
    max-height: min(22rem, 70vh);
  }

  .multiselect-options {
    display: grid;
    gap: var(--space-2);
    max-height: 14rem;
    overflow: auto;
  }

  .multiselect-option {
    width: 100%;
    justify-content: space-between;
  }

  .multiselect-option[data-selected='true'] {
    border-color: var(--success);
  }

  .multiselect-option-label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .multiselect-pills {
    align-items: flex-start;
  }

  .multiselect-pill {
    align-items: center;
    display: inline-flex;
    gap: var(--space-2);
  }

  .multiselect-pill-remove {
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    line-height: 1;
    padding: 0;
  }
</style>
