<script lang="ts">
  import { onMount } from 'svelte'
  import { deleteQuery as apiDeleteQuery, fetchQueries, type Query } from '$lib/api'
  import { formatTimestamp, toast } from '$lib/util'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import SearchInput from '$lib/components/SearchInput.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import QueryFormDialog from '$lib/components/QueryFormDialog.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'

  let loadedQueries: Query[] = []
  let currentPage = 1
  let pageCount = 1
  let totalCount = 0
  const countPerPage = 10
  let searchTerm = ''
  let error = ''
  let loading = true
  let formOpen = false
  let editingQuery: Query | null = null
  let deleteOpen = false
  let selectedQuery: Query | null = null
  let isDeleting = false

  $: queries = loadedQueries.filter((q) => {
    const search = searchTerm.trim().toLowerCase()
    return (
      !search ||
      (q.title || '').toLowerCase().includes(search) ||
      (q.description || '').toLowerCase().includes(search) ||
      (q.query || '').toLowerCase().includes(search)
    )
  })
  $: startResult = loadedQueries.length === 0 ? 0 : (currentPage - 1) * countPerPage + 1
  $: endResult = Math.min(currentPage * countPerPage, totalCount)

  onMount(loadQueries)

  async function loadQueries() {
    loading = true
    error = ''
    try {
      const data = await fetchQueries({ page: currentPage, countPerPage })
      loadedQueries = data.queries || []
      pageCount = data.page_count || 1
      totalCount = data.total_count || loadedQueries.length
    } catch (err) {
      error = (err as Error).message || 'Failed to fetch queries'
    } finally {
      loading = false
    }
  }

  async function changePage(page: number) {
    if (page > 0 && page <= pageCount) {
      currentPage = page
      await loadQueries()
    }
  }

  function openCreate() {
    editingQuery = null
    formOpen = true
  }

  function openEdit(query: Query) {
    editingQuery = query
    formOpen = true
  }

  async function handleSaved() {
    formOpen = false
    await loadQueries()
  }

  function confirmDelete(query: Query) {
    selectedQuery = query
    deleteOpen = true
  }

  async function deleteQuery() {
    if (!selectedQuery) return
    isDeleting = true
    error = ''
    try {
      await apiDeleteQuery(selectedQuery.uuid)
      deleteOpen = false
      selectedQuery = null
      await loadQueries()
    } catch (err) {
      error = (err as Error).message || 'Failed to delete query'
    } finally {
      isDeleting = false
    }
  }

  async function copyQuery(query: Query) {
    try {
      await navigator.clipboard.writeText(query.query || '')
      toast('SQL copied to clipboard', undefined, { variant: 'success', duration: 2000 })
    } catch {
      toast('Copy failed', undefined, { variant: 'danger' })
    }
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

  {#if loading}
    <Spinner />
  {:else if queries.length === 0}
    <article class="card align-center text-light">
      {searchTerm ? 'No queries match your search' : 'No queries yet. Create one to get started.'}
    </article>
  {:else}
    <div class="vstack gap-4 query-list">
      {#each queries as query (query.uuid)}
        <article class="card query-card">
          <header class="hstack justify-between query-card-head">
            <div class="vstack gap-1 query-card-title">
              <div class="hstack gap-2">
                <h3>{query.title || 'Untitled'}</h3>
                {#if query.is_system}
                  <span class="badge outline">system</span>
                {/if}
              </div>
              {#if query.description}
                <p class="text-light">{query.description}</p>
              {/if}
            </div>
            {#if query.updated_at}
              <small class="text-light">
                Updated <time datetime={query.updated_at}>{formatTimestamp(query.updated_at)}</time>
              </small>
            {/if}
          </header>

          <pre class="query-sql"><code>{query.query}</code></pre>

          <footer class="hstack justify-end">
            <menu class="buttons">
              <li>
                <button type="button" class="small outline" onclick={() => copyQuery(query)}>
                  Copy
                </button>
              </li>
              {#if !query.is_system}
                <li>
                  <button type="button" class="small outline" onclick={() => openEdit(query)}>
                    Edit
                  </button>
                </li>
                <li>
                  <button
                    type="button"
                    class="small outline"
                    data-variant="danger"
                    onclick={() => confirmDelete(query)}
                  >
                    Delete
                  </button>
                </li>
              {/if}
            </menu>
          </footer>
        </article>
      {/each}
    </div>

    <footer class="hstack justify-between">
      <p class="text-light">
        Showing <strong>{startResult}</strong> to <strong>{endResult}</strong> of
        <strong>{totalCount}</strong> results
      </p>
      <Pagination {currentPage} {pageCount} onPageChange={changePage} />
    </footer>
  {/if}
</section>

<QueryFormDialog
  open={formOpen}
  queryRecord={editingQuery}
  onClose={() => (formOpen = false)}
  onSaved={handleSaved}
/>

<ConfirmDialog
  bind:open={deleteOpen}
  title="Delete Query"
  message="Are you sure you want to delete this query? This action cannot be undone."
  confirming={isDeleting}
  confirmingLabel="Deleting..."
  onConfirm={deleteQuery}
  onCancel={() => (selectedQuery = null)}
/>

<style>
  .query-card-head {
    align-items: flex-start;
    gap: var(--space-4);
  }
  .query-card-title {
    min-width: 0;
    flex: 1 1 auto;
  }
  .query-card-title h3 {
    margin: 0;
  }
  .query-sql {
    margin: 0;
    overflow-x: auto;
  }
  .query-sql code {
    white-space: pre;
  }
</style>
