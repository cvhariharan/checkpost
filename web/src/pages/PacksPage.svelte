<script>
  import { onMount } from 'svelte'
  import { fetchPacks } from '@/api.js'
  import SearchInput from '@/components/common/SearchInput.svelte'
  import ErrorMessage from '@/components/common/ErrorMessage.svelte'

  let packs = []
  let searchTerm = ''
  let error = ''

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
    error = ''
    try {
      const data = await fetchPacks()
      packs = data.packs || data || []
    } catch (err) {
      packs = []
      error = err.message || 'Failed to fetch packs'
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
</section>
