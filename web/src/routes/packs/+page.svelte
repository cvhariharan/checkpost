<script lang="ts">
  import { onMount } from 'svelte'
  import { fetchPacks, type Pack } from '$lib/api'
  import SearchInput from '$lib/components/SearchInput.svelte'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Spinner from '$lib/components/Spinner.svelte'

  let packs: Pack[] = []
  let searchTerm = ''
  let error = ''
  let loading = true

  $: filteredPacks = packs.filter((pack) => {
    const search = searchTerm.trim().toLowerCase()
    return (
      !search ||
      (pack.Name || '').toLowerCase().includes(search) ||
      (pack.Description || '').toLowerCase().includes(search) ||
      String(pack.Queries || '').includes(search) ||
      String(pack.Targets || '').includes(search)
    )
  })

  onMount(loadPacks)

  async function loadPacks() {
    loading = true
    error = ''
    try {
      const data = await fetchPacks()
      packs = Array.isArray(data) ? data : data?.packs || []
    } catch (err) {
      packs = []
      error = (err as Error).message || 'Failed to fetch packs'
    } finally {
      loading = false
    }
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between">
    <div>
      <h1>Query Packs</h1>
      <p class="text-light">Manage osquery packs and their queries</p>
    </div>
  </header>

  <div class="row">
    <div class="col-6">
      <SearchInput bind:value={searchTerm} placeholder="Search packs..." />
    </div>
  </div>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if loading}
    <Spinner />
  {:else}
    <div class="row">
      {#each filteredPacks as pack}
        <div class="col-4">
          <article class="card">
            <header>
              <h2>{pack.Name}</h2>
              <p>{pack.Description || 'No description'}</p>
            </header>
            <div class="hstack gap-2">
              <span class="badge">{pack.Queries} queries</span>
              <span class="badge" data-variant="secondary">{pack.Targets} targets</span>
            </div>
          </article>
        </div>
      {:else}
        <div class="col-12">
          <article class="card align-center text-light">No packs found</article>
        </div>
      {/each}
    </div>
  {/if}
</section>
