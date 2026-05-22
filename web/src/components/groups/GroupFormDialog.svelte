<script>
  import { createGroup, updateGroup } from '@/api.js'
  import ErrorMessage from '@/components/common/ErrorMessage.svelte'

  export let open = false
  export let group = null
  export let onClose = () => {}
  export let onSaved = () => {}

  let dialog
  let preparedFor = null
  let title = ''
  let description = ''
  let error = ''
  let isSubmitting = false

  $: if (open && dialog) {
    const key = group?.uuid || 'new'
    if (preparedFor !== key) {
      loadForm(group)
      preparedFor = key
    }
    if (!dialog.open) dialog.showModal()
  }

  $: if (!open && dialog) {
    preparedFor = null
    if (dialog.open) dialog.close()
  }

  function loadForm(record) {
    title = record?.name || record?.title || ''
    description = record?.description || ''
    error = ''
    isSubmitting = false
  }

  function handleClose() {
    onClose()
  }

  async function saveGroup(event) {
    event.preventDefault()
    isSubmitting = true
    error = ''
    try {
      const payload = { title, description }
      if (group?.uuid) {
        await updateGroup(group.uuid, payload)
      } else {
        await createGroup(payload)
      }
      onSaved()
      dialog.close()
    } catch (err) {
      error = err.message || 'Failed to save group'
    } finally {
      isSubmitting = false
    }
  }
</script>

<dialog bind:this={dialog} onclose={handleClose} closedby="any">
  <form onsubmit={saveGroup}>
    <header>
      <h2>{group ? 'Edit Group' : 'Create Group'}</h2>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <div class="vstack">
      <label>
        Name
        <input bind:value={title} required placeholder="Group name" />
      </label>

      <label>
        Description
        <textarea bind:value={description} rows="3" placeholder="What this group represents"></textarea>
      </label>
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => dialog.close()}>Cancel</button>
      <button type="submit" disabled={isSubmitting}>
        {isSubmitting ? (group ? 'Updating...' : 'Creating...') : (group ? 'Update' : 'Create')}
      </button>
    </footer>
  </form>
</dialog>
