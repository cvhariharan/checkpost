<script lang="ts">
  type Option = { value: string; label: string }
  type RawOption = string | { value?: string; label?: string; name?: string; uuid?: string }

  let {
    label = '',
    options = [],
    value = $bindable(''),
    placeholder = 'Select an option',
    disabled = false
  }: {
    label?: string
    options?: RawOption[]
    value?: string
    placeholder?: string
    disabled?: boolean
  } = $props()

  const menuId = `select-${Math.random().toString(36).slice(2, 10)}`

  let trigger = $state<HTMLButtonElement>()
  let popover = $state<HTMLDivElement>()

  const normalizedOptions = $derived((options || []).map<Option>((option) =>
    typeof option === 'string'
      ? { value: option, label: option }
      : {
          value: option?.value ?? option?.uuid ?? '',
          label: option?.label ?? option?.name ?? option?.value ?? option?.uuid ?? ''
        }
  ))
  const selected = $derived(normalizedOptions.find((option) => option.value === value) ?? null)

  function syncMenuWidth() {
    if (!trigger || !popover) return
    popover.style.minWidth = `${trigger.getBoundingClientRect().width}px`
  }

  function selectOption(next: string) {
    value = next
    popover?.hidePopover()
  }

  function handleToggle(event: ToggleEvent) {
    if (event.newState === 'open') syncMenuWidth()
  }
</script>

<div class="selectdropdown vstack gap-2">
  {#if label}<span>{label}</span>{/if}

  <ot-dropdown class="selectdropdown-dropdown">
    <button
      bind:this={trigger}
      type="button"
      class="outline selectdropdown-trigger"
      popovertarget={menuId}
      {disabled}
      aria-label={label || placeholder}
    >
      <span>{selected ? selected.label : placeholder}</span>
      <span aria-hidden="true" class="selectdropdown-chevron">▾</span>
    </button>

    <div bind:this={popover} id={menuId} popover class="selectdropdown-menu" ontoggle={handleToggle}>
      {#each normalizedOptions as option}
        <button
          type="button"
          role="menuitem"
          class="selectdropdown-option"
          aria-current={option.value === value ? 'true' : undefined}
          onclick={() => selectOption(option.value)}
        >
          <span class="selectdropdown-option-label">{option.label}</span>
          {#if option.value === value}<span aria-hidden="true">✓</span>{/if}
        </button>
      {/each}
    </div>
  </ot-dropdown>
</div>

<style>
  .selectdropdown,
  .selectdropdown-dropdown {
    width: 100%;
  }
  .selectdropdown-trigger {
    width: 100%;
    justify-content: space-between;
  }
  .selectdropdown-chevron {
    opacity: 0.7;
  }
  .selectdropdown-menu {
    flex-direction: column;
    max-height: min(18rem, 60vh);
    overflow: auto;
    padding: var(--space-1);
    background-color: var(--card);
  }
  /* Only lay out the menu when open; a closed [popover] must keep its
     UA `display: none`, otherwise it lingers as an invisible, click-eating
     overlay parked over the trigger (oat keeps closed popovers at opacity 0). */
  .selectdropdown-menu:popover-open {
    display: flex;
  }
  .selectdropdown-option {
    justify-content: space-between;
  }
  .selectdropdown-option[aria-current='true'] {
    font-weight: var(--font-medium);
  }
  .selectdropdown-option-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
