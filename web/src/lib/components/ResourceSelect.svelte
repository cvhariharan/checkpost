<script lang="ts">
  import { tick } from 'svelte'
  import {
    fetchGroup,
    fetchGroups,
    fetchPolicies,
    fetchPolicy,
    fetchSavedQueries,
    fetchSavedQuery
  } from '$lib/api'

  type Option = { value: string; label: string }
  type Resource = 'policies' | 'groups' | 'saved-queries'

  let {
    resource,
    multiple = false,
    value = $bindable<string | string[]>(),
    label = '',
    placeholder = 'Select...',
    searchPlaceholder = 'Search...',
    emptyLabel = 'No matches',
    disabled = false
  }: {
    resource: Resource
    multiple?: boolean
    value?: string | string[]
    label?: string
    placeholder?: string
    searchPlaceholder?: string
    emptyLabel?: string
    disabled?: boolean
  } = $props()

  const menuId = `resource-select-${Math.random().toString(36).slice(2, 10)}`

  let trigger = $state<HTMLButtonElement>()
  let popover = $state<HTMLDivElement>()
  let searchInput = $state<HTMLInputElement>()
  let searchTerm = $state('')
  let results = $state<Option[]>([])
  let loading = $state(false)
  // Cache of value -> label so already-selected ids render even before a search
  // surfaces them (hydrated from the single-resource GET on edit).
  let labels = $state<Record<string, string>>({})
  let debounceTimer: ReturnType<typeof setTimeout> | undefined

  const selectedIds = $derived(
    multiple ? ((value as string[]) ?? []) : value ? [value as string] : []
  )
  const selectionSummary = $derived(
    multiple
      ? selectedIds.length === 0
        ? placeholder
        : `${selectedIds.length} selected`
      : selectedIds.length
        ? labels[selectedIds[0]] || selectedIds[0]
        : placeholder
  )

  async function search(term: string) {
    loading = true
    try {
      let opts: Option[]
      if (resource === 'policies') {
        const data = await fetchPolicies({ page: 1, countPerPage: 20, query: term })
        opts = (data.policies || []).map((p) => ({ value: p.uuid, label: p.title || p.name || p.uuid }))
      } else if (resource === 'saved-queries') {
        const data = await fetchSavedQueries({ page: 1, countPerPage: 20, query: term })
        opts = (data.saved_queries || []).map((q) => ({ value: q.id, label: q.name || q.id }))
      } else {
        const data = await fetchGroups({ page: 1, countPerPage: 20, query: term })
        opts = (data.groups || []).map((g) => ({ value: g.uuid, label: g.name || g.uuid }))
      }
      results = opts
      const next = { ...labels }
      for (const o of opts) next[o.value] = o.label
      labels = next
    } catch {
      results = []
    } finally {
      loading = false
    }
  }

  function scheduleSearch(term: string) {
    clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => search(term), 200)
  }

  $effect(() => {
    const term = searchTerm
    scheduleSearch(term)
  })

  // Resolve labels for preset ids (edit mode stores uuids only).
  async function hydrate(ids: string[]) {
    const missing = ids.filter((id) => !labels[id])
    if (!missing.length) return
    const resolved: Record<string, string> = {}
    await Promise.all(
      missing.map(async (id) => {
        try {
          if (resource === 'policies') {
            const p = await fetchPolicy(id)
            resolved[id] = p.title || p.name || id
          } else if (resource === 'saved-queries') {
            const q = await fetchSavedQuery(id)
            resolved[id] = q.name || id
          } else {
            const g = await fetchGroup(id)
            resolved[id] = g.name || id
          }
        } catch {
          resolved[id] = id
        }
      })
    )
    labels = { ...labels, ...resolved }
  }

  $effect(() => {
    if (selectedIds.length) hydrate(selectedIds)
  })

  function isSelected(id: string) {
    return selectedIds.includes(id)
  }

  function pick(option: Option) {
    labels = { ...labels, [option.value]: option.label }
    if (multiple) {
      const current = (value as string[]) ?? []
      value = current.includes(option.value)
        ? current.filter((v) => v !== option.value)
        : [...current, option.value]
      return
    }
    value = option.value
    popover?.hidePopover()
  }

  function removeOption(id: string) {
    if (multiple) {
      value = ((value as string[]) ?? []).filter((v) => v !== id)
    } else {
      value = ''
    }
  }

  function syncMenuWidth() {
    if (!trigger || !popover) return
    popover.style.width = `${Math.max(trigger.getBoundingClientRect().width, 320)}px`
  }

  async function handleToggle(event: ToggleEvent) {
    if (event.newState === 'open') {
      syncMenuWidth()
      if (results.length === 0) search(searchTerm)
      await tick()
      searchInput?.focus()
      return
    }
    searchTerm = ''
  }
</script>

<div class="resourceselect vstack gap-2">
  {#if label}<span>{label}</span>{/if}

  <ot-dropdown class="resourceselect-dropdown">
    <button
      bind:this={trigger}
      type="button"
      class="outline resourceselect-trigger"
      popovertarget={menuId}
      {disabled}
      aria-label={label || placeholder}
    >
      <span class="resourceselect-summary">{selectionSummary}</span>
      <span aria-hidden="true" class="resourceselect-chevron">▾</span>
    </button>

    <div bind:this={popover} id={menuId} popover class="card resourceselect-menu" ontoggle={handleToggle}>
      <div class="vstack gap-2">
        <input bind:this={searchInput} bind:value={searchTerm} placeholder={searchPlaceholder} />

        <div class="resourceselect-options">
          {#if loading}
            <p class="text-light">Searching…</p>
          {:else if results.length === 0}
            <p class="text-light">{emptyLabel}</p>
          {:else}
            {#each results as option (option.value)}
              <button
                type="button"
                role="menuitem"
                class="small outline resourceselect-option"
                data-selected={isSelected(option.value) ? 'true' : 'false'}
                onclick={() => pick(option)}
                aria-label={`${isSelected(option.value) ? 'Remove' : 'Add'} ${option.label}`}
              >
                <span class="resourceselect-option-label">{option.label}</span>
                {#if multiple}
                  <span class="badge" data-variant={isSelected(option.value) ? 'success' : undefined}>
                    {isSelected(option.value) ? 'Selected' : 'Add'}
                  </span>
                {:else if isSelected(option.value)}
                  <span aria-hidden="true">✓</span>
                {/if}
              </button>
            {/each}
          {/if}
        </div>
      </div>
    </div>
  </ot-dropdown>

  {#if multiple && selectedIds.length > 0}
    <div class="hstack gap-2 resourceselect-pills">
      {#each selectedIds as id (id)}
        <span class="badge outline resourceselect-pill">
          <span>{labels[id] || id}</span>
          <button
            type="button"
            class="resourceselect-pill-remove"
            aria-label={`Remove ${labels[id] || id}`}
            onclick={() => removeOption(id)}
          >
            ×
          </button>
        </span>
      {/each}
    </div>
  {/if}
</div>

<style>
  .resourceselect,
  .resourceselect-dropdown {
    width: 100%;
  }
  .resourceselect-trigger {
    width: 100%;
    justify-content: space-between;
  }
  .resourceselect-summary {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .resourceselect-chevron {
    opacity: 0.7;
  }
  .resourceselect-menu {
    max-height: min(22rem, 70vh);
  }
  .resourceselect-options {
    display: grid;
    gap: var(--space-2);
    max-height: 14rem;
    overflow: auto;
  }
  .resourceselect-option {
    width: 100%;
    justify-content: space-between;
  }
  .resourceselect-option[data-selected='true'] {
    border-color: var(--success);
  }
  .resourceselect-option-label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .resourceselect-pills {
    align-items: flex-start;
    flex-wrap: wrap;
  }
  .resourceselect-pill {
    align-items: center;
    display: inline-flex;
    gap: var(--space-2);
  }
  .resourceselect-pill-remove {
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    line-height: 1;
    padding: 0;
  }
</style>
