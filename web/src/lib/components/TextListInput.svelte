<script lang="ts">
  let {
    label = '',
    value = $bindable<string[]>(['']),
    placeholder = '',
    addLabel = 'Add',
    disabled = false,
    required = false,
    splitPaste = false
  }: {
    label?: string
    value?: string[]
    placeholder?: string
    addLabel?: string
    disabled?: boolean
    required?: boolean
    splitPaste?: boolean
  } = $props()

  $effect(() => {
    if (value && value.length > 0) return
    value = ['']
  })

  function addItem() {
    value = [...value, '']
  }

  function removeItem(index: number) {
    value = value.filter((_, i) => i !== index)
    if (value.length === 0) {
      value = ['']
    }
  }

  function updateItem(index: number, next: string) {
    value = value.map((item, i) => (i === index ? next : item))
  }

  function pasteItems(event: ClipboardEvent, index: number) {
    if (!splitPaste) return
    const text = event.clipboardData?.getData('text') || ''
    if (!/[\r\n]/.test(text)) return
    const items = text.split(/\r?\n/).map((item) => item.trim()).filter(Boolean)
    if (!items.length) return
    event.preventDefault()
    value = [...value.slice(0, index), ...items, ...value.slice(index + 1)]
  }
</script>

<div class="text-list-input">
  <div class="text-list-input-header">
    {#if label}<span>{label}{#if required}<span class="req" aria-hidden="true">*</span>{/if}</span>{/if}
    <button
      type="button"
      class="small outline icon-button"
      aria-label={addLabel}
      disabled={disabled}
      onclick={addItem}
    >
      +
    </button>
  </div>

  <div class="text-list-input-list">
    {#each value as item, index}
      <div class="text-list-input-row">
        <input
          aria-label={`${label || 'Item'} ${index + 1}`}
          value={item}
          {placeholder}
          disabled={disabled}
          oninput={(event) => updateItem(index, event.currentTarget.value)}
          onpaste={(event) => pasteItems(event, index)}
        />
        <button
          type="button"
          class="small outline icon-button"
          aria-label={`Remove ${label || 'item'}`}
          disabled={disabled || value.length === 1}
          onclick={() => removeItem(index)}
        >
          -
        </button>
      </div>
    {/each}
  </div>
</div>

<style>
  .text-list-input {
    display: grid;
    gap: var(--space-2);
    width: 100%;
  }

  .text-list-input-header,
  .text-list-input-row {
    align-items: center;
    display: grid;
    gap: var(--space-2);
    grid-template-columns: minmax(0, 1fr) 2.4rem;
  }

  .text-list-input-list {
    display: grid;
    gap: var(--space-2);
  }

  .icon-button {
    align-items: center;
    display: inline-flex;
    height: 2.4rem;
    justify-content: center;
    line-height: 1;
    min-width: 0;
    padding: var(--space-2);
    width: 2.4rem;
  }
</style>
