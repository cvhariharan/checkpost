<script lang="ts">
  import { createPolicy, fetchGroups, updatePolicy, type Group, type Policy } from '$lib/api'
  import ErrorMessage from './ErrorMessage.svelte'
  import MultiSelectDropdown from './MultiSelectDropdown.svelte'
  import SelectDropdown from './SelectDropdown.svelte'

  export let open = false
  export let policy: Policy | null = null
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  const platformOptions = [
    { value: 'all', label: 'All' },
    { value: 'any', label: 'Any' },
    { value: 'posix', label: 'POSIX' },
    { value: 'darwin', label: 'macOS' },
    { value: 'linux', label: 'Linux' },
    { value: 'windows', label: 'Windows' }
  ]

  let dialog: HTMLDialogElement
  let preparedFor: string | null = null
  let title = ''
  let query = ''
  let description = ''
  let resolution = ''
  let platform = 'all'
  let enabled = true
  let groupIds: string[] = []
  let availableGroups: Group[] = []
  let error = ''
  let isSubmitting = false

  $: if (open && dialog) {
    const key = policy?.uuid || 'new'
    if (preparedFor !== key) {
      loadForm(policy)
      preparedFor = key
      loadGroups()
    }
    if (!dialog.open) dialog.showModal()
  }

  $: if (!open && dialog) {
    preparedFor = null
    if (dialog.open) dialog.close()
  }

  function loadForm(record: Policy | null) {
    title = record?.title || record?.name || ''
    query = record?.query || ''
    description = record?.description || ''
    resolution = record?.resolution || ''
    platform = record?.platform || 'all'
    enabled = record?.enabled ?? true
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

  async function savePolicy(event: SubmitEvent) {
    event.preventDefault()
    isSubmitting = true
    error = ''
    try {
      const payload = { title, query, description, resolution, platform, enabled, group_ids: groupIds }
      if (policy?.uuid) await updatePolicy(policy.uuid, payload)
      else await createPolicy(payload)
      onSaved()
      dialog.close()
    } catch (err) {
      error = (err as Error).message || 'Failed to save policy'
    } finally {
      isSubmitting = false
    }
  }
</script>

<dialog bind:this={dialog} onclose={handleClose} closedby="any">
  <form onsubmit={savePolicy}>
    <header>
      <h2>{policy ? 'Edit Policy' : 'Create Policy'}</h2>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <div class="vstack">
      <label data-field>
        Name
        <input bind:value={title} required placeholder="Policy name" />
      </label>

      <label data-field>
        Query
        <textarea
          bind:value={query}
          required
          rows="7"
          placeholder="SELECT 1 FROM disk_encryption WHERE encrypted = 1;"
        ></textarea>
        <small data-hint
          >Return <code>1</code> when the host passes, <code>0</code> (or no rows) when it fails.</small
        >
      </label>

      <label data-field>
        Description
        <textarea bind:value={description} rows="3" placeholder="What this policy checks"></textarea>
      </label>

      <label data-field>
        Resolution
        <textarea bind:value={resolution} rows="3" placeholder="How to resolve failing machines"></textarea>
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
        <input type="checkbox" bind:checked={enabled} />
        Enabled
      </label>
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => dialog.close()}>Cancel</button>
      <button type="submit" disabled={isSubmitting} aria-busy={isSubmitting ? 'true' : undefined}>
        {isSubmitting
          ? policy
            ? 'Updating...'
            : 'Creating...'
          : policy
            ? 'Update'
            : 'Create'}
      </button>
    </footer>
  </form>
</dialog>
