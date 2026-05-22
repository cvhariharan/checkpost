<script>
  import { onMount } from 'svelte'
  import { deleteQuery as apiDeleteQuery, fetchQueries } from '@/api.js'
  import ErrorMessage from '@/components/common/ErrorMessage.svelte'
  import OatPagination from '@/components/common/OatPagination.svelte'
  import SearchInput from '@/components/common/SearchInput.svelte'
  import QueryFormDialog from '@/components/queries/QueryFormDialog.svelte'

  let loadedQueries = []
  let currentPage = 1
  let pageCount = 1
  let totalCount = 0
  let countPerPage = 10
  let searchTerm = ''
  let error = ''
  let copiedQuery = null
  let copyTimeout = null
  let formOpen = false
  let editingQuery = null
  let deleteDialog
  let selectedQuery = null
  let isDeleting = false

  $: queries = loadedQueries.filter((query) => {
    const search = searchTerm.trim().toLowerCase()
    return (
      !search ||
      (query.title || '').toLowerCase().includes(search) ||
      (query.description || '').toLowerCase().includes(search) ||
      (query.query || '').toLowerCase().includes(search)
    )
  })
  $: startResult = loadedQueries.length === 0 ? 0 : (currentPage - 1) * countPerPage + 1
  $: endResult = Math.min(currentPage * countPerPage, totalCount)

  onMount(loadQueries)

  async function loadQueries() {
    error = ''
    try {
      const data = await fetchQueries({ page: currentPage, countPerPage })
      loadedQueries = data.queries || []
      pageCount = data.page_count || 1
      totalCount = data.total_count || loadedQueries.length
    } catch (err) {
      error = err.message || 'Failed to fetch queries'
    }
  }

  async function changePage(page) {
    if (page > 0 && page <= pageCount) {
      currentPage = page
      await loadQueries()
    }
  }

  function openCreate() {
    editingQuery = null
    formOpen = true
  }

  function openEdit(query) {
    editingQuery = query
    formOpen = true
  }

  async function handleSaved() {
    formOpen = false
    await loadQueries()
  }

  function confirmDelete(query) {
    selectedQuery = query
    deleteDialog.showModal()
  }

  async function deleteQuery() {
    if (!selectedQuery) return
    isDeleting = true
    error = ''
    try {
      await apiDeleteQuery(selectedQuery.uuid)
      deleteDialog.close()
      selectedQuery = null
      await loadQueries()
    } catch (err) {
      error = err.message || 'Failed to delete query'
    } finally {
      isDeleting = false
    }
  }

  function copyQuery(query) {
    const sql = query.query || ''
    if (navigator.clipboard) {
      navigator.clipboard.writeText(sql)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = sql
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }

    copiedQuery = query.uuid
    if (copyTimeout) clearTimeout(copyTimeout)
    copyTimeout = setTimeout(() => {
      copiedQuery = null
    }, 2000)
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between">
    <div>
      <h1>Stored Queries</h1>
      <p class="text-light">Manage and organize your osquery queries</p>
    </div>
    <button type="button" onclick={openCreate}>Create Query</button>
  </header>

  <div class="row">
    <div class="col-6">
      <SearchInput bind:value={searchTerm} placeholder="Search queries..." />
    </div>
  </div>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  <div class="table">
    <table>
      <thead>
        <tr>
          <th>Title</th>
          <th>SQL</th>
          <th class="align-right">Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each queries as query}
          <tr>
            <td>
              <strong>{query.title || 'Untitled'}</strong>
              {#if query.description}
                <p class="text-light">{query.description}</p>
              {/if}
            </td>
            <td><pre><code>{query.query}</code></pre></td>
            <td class="align-right">
              <div class="hstack justify-end gap-2">
                <button type="button" class="small outline" onclick={() => copyQuery(query)}>
                  {copiedQuery === query.uuid ? 'Copied' : 'Copy'}
                </button>
                <button type="button" class="small outline" onclick={() => openEdit(query)}>Edit</button>
                <button type="button" class="small outline" data-variant="danger" onclick={() => confirmDelete(query)}>Delete</button>
              </div>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="3" class="align-center text-light">No queries found</td>
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

<QueryFormDialog
  open={formOpen}
  queryRecord={editingQuery}
  onClose={() => (formOpen = false)}
  onSaved={handleSaved}
/>

<dialog bind:this={deleteDialog} closedby="any">
  <form method="dialog">
    <header>
      <h2>Delete Query</h2>
      <p>Are you sure you want to delete this query? This action cannot be undone.</p>
    </header>
    <footer>
      <button type="button" class="outline" onclick={() => deleteDialog.close()}>Cancel</button>
      <button type="button" data-variant="danger" disabled={isDeleting} onclick={deleteQuery}>
        {isDeleting ? 'Deleting...' : 'Delete'}
      </button>
    </footer>
  </form>
</dialog>
