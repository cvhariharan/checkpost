<script lang="ts">
  import {
    createAlertRule,
    fetchAlertSources,
    fetchAlertTargets,
    updateAlertRule,
    type AlertRule,
    type AlertSource,
    type AlertTarget
  } from '$lib/api'
  import ErrorMessage from './ErrorMessage.svelte'
  import MultiSelectDropdown from './MultiSelectDropdown.svelte'
  import ResourceSelect from './ResourceSelect.svelte'
  import SelectDropdown from './SelectDropdown.svelte'
  import TextListInput from './TextListInput.svelte'

  // Widgets whose params round-trip as a string[] (stored in listValues).
  // resource-select is multi (array) in every current alert source schema.
  const LIST_WIDGETS = ['text-list', 'multiselect', 'resource-select']

  let {
    open = false,
    rule = null,
    onClose = () => {},
    onSaved = () => {}
  }: {
    open?: boolean
    rule?: AlertRule | null
    onClose?: () => void
    onSaved?: () => void
  } = $props()

  type Field = {
    name: string
    type: string
    itemType: string
    title: string
    description: string
    widget: string
    resource: string
    placeholder: string
    options: string[]
  }

  let dialog = $state<HTMLDialogElement>()
  let preparedFor = $state<string | null>(null)
  let sources = $state<AlertSource[]>([])
  let targets = $state<AlertTarget[]>([])

  let name = $state('')
  let description = $state('')
  let source = $state('')
  let severity = $state('medium')
  let evalInterval = $state(300)
  let forSeconds = $state(0)
  let repeatSeconds = $state(0)
  let enabled = $state(true)
  let targetIds = $state<string[]>([])
  // Scalar/textarea params keyed by name; text-list (UUID list) params live in listValues.
  let paramValues = $state<Record<string, string | boolean>>({})
  let listValues = $state<Record<string, string[]>>({})
  let error = $state('')
  let isSubmitting = $state(false)

  const isEdit = $derived(!!rule?.uuid)
  const fields = $derived(fieldsFor(source))
  const targetOptions = $derived(targets.map((t) => ({ value: t.uuid, label: t.name || t.uuid })))

  $effect(() => {
    if (!open || !dialog) return
    const key = rule?.uuid || 'new'
    if (preparedFor !== key) {
      prepare()
      preparedFor = key
    }
    if (!dialog.open) dialog.showModal()
  })

  $effect(() => {
    if (open || !dialog) return
    preparedFor = null
    if (dialog.open) dialog.close()
  })

  async function prepare() {
    error = ''
    isSubmitting = false
    name = rule?.name || ''
    description = rule?.description || ''
    severity = rule?.severity || 'medium'
    evalInterval = rule?.evaluation_interval_seconds || 300
    forSeconds = rule?.for_seconds || 0
    repeatSeconds = rule?.repeat_interval_seconds || 0
    enabled = rule?.enabled ?? true
    targetIds = rule?.target_ids ? [...rule.target_ids] : []

    try {
      const [s, t] = await Promise.all([
        fetchAlertSources(),
        fetchAlertTargets({ page: 1, countPerPage: 1000 })
      ])
      sources = s.sources || []
      targets = t.targets || []
    } catch (err) {
      error = (err as Error).message || 'Failed to load sources'
    }

    source = rule?.source || sources[0]?.type || ''
    loadParams(rule?.params || {})
  }

  function fieldsFor(srcType: string): Field[] {
    const schema = sources.find((s) => s.type === srcType)?.schema as
      | { properties?: Record<string, Record<string, unknown>> }
      | undefined
    const props = schema?.properties || {}
    return Object.entries(props).map(([key, def]) => {
      const items = def.items as Record<string, unknown> | undefined
      return {
        name: key,
        type: (def.type as string) || 'string',
        itemType: (items?.type as string) || 'string',
        title: (def.title as string) || key,
        description: (def.description as string) || '',
        widget: (def['x-widget'] as string) || '',
        resource: (def['x-resource'] as string) || '',
        placeholder: (def['x-placeholder'] as string) || '',
        options: Array.isArray(items?.enum) ? (items!.enum as string[]) : []
      }
    })
  }

  function loadParams(params: Record<string, unknown>) {
    const scalars: Record<string, string | boolean> = {}
    const lists: Record<string, string[]> = {}
    for (const f of fieldsFor(source)) {
      const val = params[f.name]
      if (LIST_WIDGETS.includes(f.widget)) {
        lists[f.name] = Array.isArray(val) ? val.map(String) : []
      } else if (f.type === 'array') {
        scalars[f.name] = Array.isArray(val) ? val.join('\n') : ''
      } else if (f.type === 'boolean') {
        scalars[f.name] = !!val
      } else {
        scalars[f.name] = val === undefined ? '' : String(val)
      }
    }
    paramValues = scalars
    listValues = lists
  }

  function buildParams(): Record<string, unknown> {
    const out: Record<string, unknown> = {}
    for (const f of fields) {
      if (LIST_WIDGETS.includes(f.widget)) {
        const items = (listValues[f.name] || []).map((v) => v.trim()).filter(Boolean)
        if (items.length) out[f.name] = items
        continue
      }
      const raw = paramValues[f.name]
      if (f.type === 'boolean') {
        if (raw) out[f.name] = true
      } else if (f.type === 'array') {
        const items = String(raw || '')
          .split(/[\n,]/)
          .map((v) => v.trim())
          .filter(Boolean)
        if (items.length) out[f.name] = items
      } else if (f.type === 'integer' || f.type === 'number') {
        if (raw !== undefined && raw !== '') out[f.name] = Number(raw)
      } else if (raw) {
        out[f.name] = raw
      }
    }
    return out
  }

  async function save(event: SubmitEvent) {
    event.preventDefault()
    isSubmitting = true
    error = ''
    try {
      const payload: Record<string, unknown> = {
        name,
        description,
        source,
        params: buildParams(),
        severity,
        enabled,
        evaluation_interval_seconds: Number(evalInterval),
        for_seconds: Number(forSeconds),
        repeat_interval_seconds: Number(repeatSeconds),
        target_ids: targetIds
      }
      if (isEdit && rule) await updateAlertRule(rule.uuid, payload)
      else await createAlertRule(payload)
      onSaved()
      dialog?.close()
    } catch (err) {
      error = (err as Error).message || 'Failed to save rule'
    } finally {
      isSubmitting = false
    }
  }
</script>

<dialog bind:this={dialog} onclose={onClose} closedby="any">
  <form onsubmit={save}>
    <header>
      <h2>{isEdit ? 'Edit Alert Rule' : 'Create Alert Rule'}</h2>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <div class="vstack">
      <label data-field>
        Name <span class="req" aria-hidden="true">*</span>
        <input bind:value={name} required placeholder="Rule name" />
      </label>

      <label data-field>
        Description
        <textarea bind:value={description} rows="2"></textarea>
      </label>

      <div class="vstack gap-1">
        <SelectDropdown
          label="Source"
          options={sources.map((s) => s.type)}
          bind:value={source}
          placeholder="Select source"
          disabled={isEdit}
        />
      </div>

      {#if source && fields.length === 0}
        <small class="text-light">This source has no options.</small>
      {/if}

      {#each fields as field (field.name)}
        {#if field.widget === 'resource-select'}
          <div class="vstack gap-1">
            <ResourceSelect
              resource={(field.resource as 'policies' | 'groups') || 'policies'}
              multiple
              label={field.title}
              bind:value={listValues[field.name]}
              placeholder={`Select ${field.title.toLowerCase()}`}
              searchPlaceholder={`Search ${field.title.toLowerCase()}...`}
            />
            {#if field.description}<small class="text-light">{field.description}</small>{/if}
          </div>
        {:else if field.widget === 'multiselect'}
          <div class="vstack gap-1">
            <MultiSelectDropdown
              label={field.title}
              options={field.options.map((o) => ({ value: o, label: o }))}
              bind:value={listValues[field.name]}
              placeholder={`Select ${field.title.toLowerCase()}`}
            />
            {#if field.description}<small class="text-light">{field.description}</small>{/if}
          </div>
        {:else if field.widget === 'text-list'}
          <div class="vstack gap-1">
            <TextListInput
              label={field.title}
              bind:value={listValues[field.name]}
              placeholder={field.placeholder}
              addLabel={`Add ${field.title}`}
            />
            {#if field.description}<small class="text-light">{field.description}</small>{/if}
          </div>
        {:else}
          <label data-field>
            {field.title}
            {#if field.type === 'boolean'}
              <input type="checkbox" checked={!!paramValues[field.name]} onchange={(e) => (paramValues[field.name] = e.currentTarget.checked)} />
            {:else if field.type === 'array'}
              <textarea
                rows="2"
                value={String(paramValues[field.name] ?? '')}
                oninput={(e) => (paramValues[field.name] = e.currentTarget.value)}
                placeholder="One per line"
              ></textarea>
            {:else if field.type === 'integer' || field.type === 'number'}
              <input
                type="number"
                value={String(paramValues[field.name] ?? '')}
                oninput={(e) => (paramValues[field.name] = e.currentTarget.value)}
              />
            {:else}
              <input
                value={String(paramValues[field.name] ?? '')}
                oninput={(e) => (paramValues[field.name] = e.currentTarget.value)}
              />
            {/if}
            {#if field.description}<small class="text-light">{field.description}</small>{/if}
          </label>
        {/if}
      {/each}

      <div data-field>
        <SelectDropdown
          label="Severity"
          options={[
            { value: 'critical', label: 'Critical' },
            { value: 'high', label: 'High' },
            { value: 'medium', label: 'Medium' },
            { value: 'low', label: 'Low' },
            { value: 'info', label: 'Info' }
          ]}
          bind:value={severity}
        />
      </div>

      <div class="row">
        <label data-field class="col-6">
          Evaluation interval (s)
          <input type="number" min="60" bind:value={evalInterval} />
        </label>
        <label data-field class="col-6">
          For (s)
          <input type="number" min="0" bind:value={forSeconds} />
        </label>
      </div>

      <div class="row">
        <label data-field class="col-6">
          Repeat (s)
          <input type="number" min="0" bind:value={repeatSeconds} />
        </label>
      </div>

      <div class="vstack gap-2">
        <span>Targets</span>
        <MultiSelectDropdown
          label=""
          options={targetOptions}
          bind:value={targetIds}
          placeholder="Select targets"
          emptyLabel="No targets configured"
        />
      </div>

      <label data-field class="hstack gap-2">
        <input type="checkbox" bind:checked={enabled} />
        Enabled
      </label>
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => dialog?.close()}>Cancel</button>
      <button type="submit" class="gap-1" disabled={isSubmitting} aria-busy={isSubmitting ? 'true' : undefined} data-spinner="small">
        {isSubmitting ? 'Saving...' : isEdit ? 'Update' : 'Create'}
      </button>
    </footer>
  </form>
</dialog>
