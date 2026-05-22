<script>
  import { onMount } from 'svelte'
  import { deleteGroup as apiDeleteGroup, fetchGroups } from '@/api.js'
  import ErrorMessage from '@/components/common/ErrorMessage.svelte'
  import OatPagination from '@/components/common/OatPagination.svelte'
  import SearchInput from '@/components/common/SearchInput.svelte'
  import GroupFormDialog from '@/components/groups/GroupFormDialog.svelte'

  let loadedGroups = []
  let currentPage = 1
  let pageCount = 1
  let totalCount = 0
  let countPerPage = 10
  let searchTerm = ''
  let error = ''
  let formOpen = false
  let editingGroup = null
  let deleteDialog
  let selectedGroup = null
  let isDeleting = false

  $: groups = loadedGroups.filter((group) => {
    const search = searchTerm.trim().toLowerCase()
    return (
      !search ||
      (group.name || '').toLowerCase().includes(search) ||
      (group.description || '').toLowerCase().includes(search)
    )
  })
  $: startResult = loadedGroups.length === 0 ? 0 : (currentPage - 1) * countPerPage + 1
  $: endResult = Math.min(currentPage * countPerPage, totalCount)

  onMount(loadGroups)

  async function loadGroups() {
    error = ''
    try {
      const data = await fetchGroups({ page: currentPage, countPerPage })
      loadedGroups = data.groups || []
      pageCount = data.page_count || 1
      totalCount = data.total_count || loadedGroups.length
    } catch (err) {
      error = err.message || 'Failed to fetch groups'
    }
  }

  async function changePage(page) {
    if (page > 0 && page <= pageCount) {
      currentPage = page
      await loadGroups()
    }
  }

  function openCreate() {
    editingGroup = null
    formOpen = true
  }

  function openEdit(group) {
    editingGroup = group
    formOpen = true
  }

  async function handleSaved() {
    formOpen = false
    await loadGroups()
  }

  function confirmDelete(group) {
    selectedGroup = group
    deleteDialog.showModal()
  }

  async function deleteGroup() {
    if (!selectedGroup) return
    isDeleting = true
    error = ''
    try {
      await apiDeleteGroup(selectedGroup.uuid)
      deleteDialog.close()
      selectedGroup = null
      await loadGroups()
    } catch (err) {
      error = err.message || 'Failed to delete group'
    } finally {
      isDeleting = false
    }
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between">
    <div>
      <h1>Groups</h1>
      <p class="text-light">Organize machines and target policies to specific collections</p>
    </div>
    <button type="button" onclick={openCreate}>Create Group</button>
  </header>

  <div class="row">
    <div class="col-6">
      <SearchInput bind:value={searchTerm} placeholder="Search groups..." />
    </div>
  </div>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  <div class="table">
    <table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Description</th>
          <th class="align-right">Machines</th>
          <th class="align-right">Policies</th>
          <th class="align-right">Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each groups as group}
          <tr>
            <td><strong>{group.name || 'Untitled'}</strong></td>
            <td>{group.description || ''}</td>
            <td class="align-right">{group.machine_count || 0}</td>
            <td class="align-right">{group.policy_count || 0}</td>
            <td class="align-right">
              <div class="hstack justify-end gap-2">
                <button type="button" class="small outline" onclick={() => openEdit(group)}>Edit</button>
                <button type="button" class="small outline" data-variant="danger" onclick={() => confirmDelete(group)}>Delete</button>
              </div>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="5" class="align-center text-light">No groups found</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>

  <footer class="hstack justify-between">
    <p class="text-light">Showing <strong>{startResult}</strong> to <strong>{endResult}</strong> of <strong>{totalCount}</strong> results</p>
    <OatPagination {currentPage} {pageCount} onPageChange={changePage} />
  </footer>
</section>

<GroupFormDialog
  open={formOpen}
  group={editingGroup}
  onClose={() => (formOpen = false)}
  onSaved={handleSaved}
/>

<dialog bind:this={deleteDialog} closedby="any">
  <form method="dialog">
    <header>
      <h2>Delete Group</h2>
      <p>Are you sure you want to delete this group? Machines and policies will be detached automatically.</p>
    </header>
    <footer>
      <button type="button" class="outline" onclick={() => deleteDialog.close()}>Cancel</button>
      <button type="button" data-variant="danger" disabled={isDeleting} onclick={deleteGroup}>
        {isDeleting ? 'Deleting...' : 'Delete'}
      </button>
    </footer>
  </form>
</dialog>
