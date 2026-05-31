<script lang="ts">
  import { formatTimestamp } from '$lib/util'
  import {
    formatScalar,
    isPills,
    primaryProperty,
    progressFor,
    rootShape,
    type JSONSchema,
    type MetricEntry
  } from '$lib/metricSchema'

  let {
    kind,
    schema = undefined,
    entry = undefined
  }: {
    kind: string
    schema?: JSONSchema
    entry?: MetricEntry
  } = $props()

  const title = $derived(schema?.title || kind)
  const shape = $derived(rootShape(schema))
  const value = $derived((entry?.value ?? {}) as Record<string, unknown>)
  const tableRows = $derived(
    shape.kind === 'table' ? ((value[shape.arrayProp] as unknown[]) || []) : []
  )
  const cardProps = $derived(shape.kind === 'card' ? shape.properties : [])
  const primary = $derived(shape.kind === 'card' ? primaryProperty(cardProps) : null)
</script>

{#if entry && schema}
  <article class="card vstack gap-2">
    <header class="hstack justify-between metric-header">
      <h3>{title}</h3>
      {#if entry.collected_at}
        <small class="text-light">Updated {formatTimestamp(entry.collected_at)}</small>
      {/if}
    </header>

    {#if shape.kind === 'table'}
      {@const columns = Object.entries(shape.itemSchema.properties || {})}
      <div class="table">
        <table>
          <thead>
            <tr>
              {#each columns as [, sub]}
                <th>{sub.title}</th>
              {/each}
            </tr>
          </thead>
          <tbody>
            {#each tableRows as row}
              {@const r = row as Record<string, unknown>}
              <tr>
                {#each columns as [name, sub]}
                  <td>
                    {#if progressFor(sub, r[name]) !== null}
                      {@const used = progressFor(sub, r[name]) as number}
                      <meter class="metric-bar" min="0" max="100" low="75" high="90" optimum="0" value={used} aria-hidden="true"></meter>
                      <small class="text-light">{formatScalar(sub, r[name])}</small>
                    {:else}
                      {formatScalar(sub, r[name])}
                    {/if}
                  </td>
                {/each}
              </tr>
            {:else}
              <tr><td colspan={columns.length} class="align-center text-light">No data</td></tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <div class="vstack gap-2">
        {#if primary}
          <strong class="metric-headline">{formatScalar(cardProps.find(([n]) => n === primary)?.[1], value[primary])}</strong>
        {/if}
        <dl class="metric-fields">
          {#each cardProps as [name, sub]}
            {#if name !== primary && value[name] !== undefined && value[name] !== null}
              <div class="metric-field">
                <dt class="text-light">{sub.title || name}</dt>
                <dd>
                  {#if isPills(sub) && Array.isArray(value[name])}
                    <span class="hstack gap-1">
                      {#each value[name] as item}
                        <span class="badge secondary">{String(item)}</span>
                      {/each}
                    </span>
                  {:else if progressFor(sub, value[name]) !== null}
                    {@const used = progressFor(sub, value[name]) as number}
                    <meter class="metric-bar" min="0" max="100" low="75" high="90" optimum="0" value={used} aria-hidden="true"></meter>
                    <small class="text-light">{formatScalar(sub, value[name])}</small>
                  {:else}
                    {formatScalar(sub, value[name])}
                  {/if}
                </dd>
              </div>
            {/if}
          {/each}
        </dl>
      </div>
    {/if}
  </article>
{/if}

<style>
  .metric-header {
    align-items: baseline;
    flex-wrap: wrap;
  }
  .metric-header h3 {
    margin: 0;
    font-size: var(--text-6);
  }
  .metric-headline {
    font-size: var(--text-4);
    line-height: 1.2;
    word-break: break-word;
  }
  .metric-fields {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: var(--space-1) var(--space-3);
    margin: 0;
    font-size: var(--text-7);
  }
  .metric-field {
    display: contents;
  }
  .metric-field dt {
    margin: 0;
    align-self: baseline;
  }
  .metric-field dd {
    margin: 0;
    word-break: break-word;
  }
  .metric-bar {
    max-width: 14rem;
  }
</style>
