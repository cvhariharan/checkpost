<script lang="ts">
  import {
    createSchedule,
    fetchGroups,
    fetchSchedule,
    updateSchedule,
    type Group,
    type Schedule
  } from '$lib/api'
  import ErrorMessage from './ErrorMessage.svelte'
  import MultiSelectDropdown from './MultiSelectDropdown.svelte'
  import SelectDropdown from './SelectDropdown.svelte'
  import SqlEditor from './SqlEditor.svelte'

  const platformOptions = [
    { value: 'all', label: 'All' },
    { value: 'any', label: 'Any' },
    { value: 'posix', label: 'POSIX' },
    { value: 'darwin', label: 'macOS' },
    { value: 'linux', label: 'Linux' },
    { value: 'windows', label: 'Windows' }
  ]

  let {
    open = false,
    schedule = null,
    onClose = () => {},
    onSaved = () => {}
  }: {
    open?: boolean
    schedule?: Schedule | null
    onClose?: () => void
    onSaved?: () => void
  } = $props()

  let dialog = $state<HTMLDialogElement>()
  let formEl = $state<HTMLFormElement>()
  let preparedFor = $state<string | null>(null)
  let availableGroups = $state<Group[]>([])
  let query = $state('')
  let description = $state('')
  let title = $state('')
  let interval = $state(3600)
  let platform = $state('all')
  let snapshot = $state(false)
  let groupIds = $state<string[]>([])
  let error = $state('')
  let isSubmitting = $state(false)

  // System schedules are built-in: only their run interval may be changed.
  const isSystem = $derived(Boolean(schedule?.is_system))

  $effect(() => {
    if (!open || !dialog) return
    const key = schedule?.uuid || 'new'
    if (preparedFor !== key) {
      preparedFor = key
      void prepareForm(schedule)
      loadGroups()
    }
    if (!dialog.open) dialog.showModal()
  })

  async function prepareForm(record: Schedule | null) {
    loadForm(record)
    if (!record?.uuid) return
    try {
      const fresh = await fetchSchedule(record.uuid)
      // Bail if the dialog has since switched to a different record.
      if (preparedFor === record.uuid) loadForm(fresh)
    } catch {
      // Keep the snapshot-seeded values if the refresh fails.
    }
  }

  $effect(() => {
    if (open || !dialog) return
    preparedFor = null
    if (dialog.open) dialog.close()
  })

  function loadForm(record: Schedule | null) {
    query = record?.sql || ''
    description = record?.description || ''
    title = record?.title || ''
    interval = record?.interval || 3600
    platform = record?.platform || 'all'
    snapshot = Boolean(record?.snapshot)
    groupIds = (record?.groups || []).map((g) => g.uuid)
    error = ''
    isSubmitting = false
  }

  async function loadGroups() {
    try {
      const data = await fetchGroups({ page: 1, countPerPage: 1000 })
      availableGroups = data.groups || []
    } catch (err) {
      error = (err as Error).message || 'Failed to load groups'
    }
  }

  function handleClose() {
    onClose()
  }

  async function saveSchedule(event: SubmitEvent) {
    event.preventDefault()
    isSubmitting = true
    error = ''
    if (!query.trim()) {
      error = 'Query is required.'
      isSubmitting = false
      return
    }
    try {
      const payload = {
        query,
        description,
        title,
        interval: Number.parseInt(String(interval), 10),
        platform,
        snapshot,
        group_ids: groupIds
      }
      if (schedule?.uuid) await updateSchedule(schedule.uuid, payload)
      else await createSchedule(payload)
      onSaved()
      dialog?.close()
    } catch (err) {
      error = (err as Error).message || 'Failed to save schedule'
    } finally {
      isSubmitting = false
    }
  }
</script>

<dialog bind:this={dialog} onclose={handleClose} closedby="any">
  <form bind:this={formEl} onsubmit={saveSchedule}>
    <header>
      <h2>{schedule ? (isSystem ? 'Edit System Schedule' : 'Edit Schedule') : 'Create Schedule'}</h2>
    </header>

    <div class="vstack">
      <ErrorMessage message={error} onClose={() => (error = '')} />

      {#if isSystem}
        <small data-hint>Only the interval can be changed for system schedules.</small>
      {/if}

      <label data-field>
        Title <span class="req" aria-hidden="true">*</span>
        <input bind:value={title} required placeholder="Schedule title" disabled={isSystem} />
      </label>

      <label data-field>
        Query <span class="req" aria-hidden="true">*</span>
        <SqlEditor
          bind:value={query}
          minLines={4}
          placeholder="SELECT * FROM ..."
          ariaLabel="Schedule SQL query"
          disabled={isSystem}
          onsubmit={() => formEl?.requestSubmit()}
        />
      </label>

      <label data-field>
        Description
        <input bind:value={description} placeholder="Optional description" disabled={isSystem} />
      </label>

      <label data-field>
        Interval (seconds) <span class="req" aria-hidden="true">*</span>
        <input type="number" bind:value={interval} required min="1" max="604800" />
      </label>

      <div data-field>
        <SelectDropdown label="Platform" options={platformOptions} bind:value={platform} disabled={isSystem} />
      </div>

      <div data-field class="vstack gap-2">
        <span>Targets</span>
        <small data-hint
          >{groupIds.length === 0 ? 'All machines for this platform' : `${groupIds.length} groups selected`}</small
        >
        <MultiSelectDropdown
          label="Groups"
          options={availableGroups}
          bind:value={groupIds}
          placeholder="All machines for this platform"
          searchPlaceholder="Search groups..."
          emptyLabel="No groups available yet"
          disabled={isSystem}
        />
      </div>

      <label>
        <input type="checkbox" bind:checked={snapshot} disabled={isSystem} />
        Snapshot
      </label>
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => dialog?.close()}>Cancel</button>
      <button type="submit" class="gap-1" disabled={isSubmitting} aria-busy={isSubmitting ? 'true' : undefined} data-spinner="small">
        {isSubmitting
          ? schedule
            ? 'Updating...'
            : 'Creating...'
          : schedule
            ? 'Update'
            : 'Create'}
      </button>
    </footer>
  </form>
</dialog>
