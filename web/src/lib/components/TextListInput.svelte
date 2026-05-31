<script lang="ts">
  export let label = ''
  export let value: string[] = ['']
  export let placeholder = ''
  export let addLabel = 'Add'
  export let disabled = false

  $: if (!value || value.length === 0) {
    value = ['']
  }

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
</script>

<div class="text-list-input">
  <div class="text-list-input-header">
    {#if label}<span>{label}</span>{/if}
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
          value={item}
          {placeholder}
          disabled={disabled}
          oninput={(event) => updateItem(index, event.currentTarget.value)}
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
