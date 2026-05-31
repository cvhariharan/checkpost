<script lang="ts">
  import {
    createSchedule,
    fetchGroups,
    updateSchedule,
    type Group,
    type Schedule
  } from '$lib/api'
  import ErrorMessage from './ErrorMessage.svelte'
  import MultiSelectDropdown from './MultiSelectDropdown.svelte'
  import SelectDropdown from './SelectDropdown.svelte'

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

  $effect(() => {
    if (!open || !dialog) return
    const key = schedule?.uuid || 'new'
    if (preparedFor !== key) {
      loadForm(schedule)
      loadGroups()
      preparedFor = key
    }
    if (!dialog.open) dialog.showModal()
  })

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
  <form onsubmit={saveSchedule}>
    <header>
      <h2>{schedule ? 'Edit Schedule' : 'Create Schedule'}</h2>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <div class="vstack">
      <label data-field>
        Title
        <input bind:value={title} required placeholder="Schedule title" />
      </label>

      <label data-field>
        Query
        <textarea bind:value={query} required rows="4" placeholder="SELECT * FROM ..."></textarea>
      </label>

      <label data-field>
        Description
        <input bind:value={description} placeholder="Optional description" />
      </label>

      <label data-field>
        Interval (seconds)
        <input type="number" bind:value={interval} required min="1" max="604800" />
      </label>

      <div data-field>
        <SelectDropdown label="Platform" options={platformOptions} bind:value={platform} />
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
        />
      </div>

      <label>
        <input type="checkbox" bind:checked={snapshot} />
        Snapshot
      </label>
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => dialog?.close()}>Cancel</button>
      <button type="submit" disabled={isSubmitting} aria-busy={isSubmitting ? 'true' : undefined}>
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
