<script lang="ts">
  import { formatTimestamp } from '$lib/util'
  import {
    formatScalar,
    isPills,
    primaryProperty,
    progressFor,
    rootShape,
    usageVariant,
    type JSONSchema,
    type MetricEntry
  } from '$lib/metricSchema'

  export let kind: string
  export let schema: JSONSchema | undefined
  export let entry: MetricEntry | undefined

  $: title = schema?.title || kind
  $: shape = rootShape(schema)
  $: value = (entry?.value ?? {}) as Record<string, unknown>
  $: tableRows =
    shape.kind === 'table' ? ((value[shape.arrayProp] as unknown[]) || []) : []
  $: cardProps = shape.kind === 'card' ? shape.properties : []
  $: primary = shape.kind === 'card' ? primaryProperty(cardProps) : null
</script>

{#if entry && schema}
  <article class="card metric">
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
                      <div class="metric-bar" data-variant={usageVariant(used)}>
                        <span style="width: {used.toFixed(1)}%"></span>
                      </div>
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
                    {#each value[name] as item}
                      <code class="metric-pill">{String(item)}</code>
                    {/each}
                  {:else if progressFor(sub, value[name]) !== null}
                    {@const used = progressFor(sub, value[name]) as number}
                    <div class="metric-bar" data-variant={usageVariant(used)}>
                      <span style="width: {used.toFixed(1)}%"></span>
                    </div>
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
  .metric {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .metric-header {
    align-items: baseline;
    gap: 0.75rem;
    flex-wrap: wrap;
  }
  .metric-header h3 {
    margin: 0;
    font-size: 1rem;
  }
  .metric-header small {
    font-size: 0.75rem;
  }
  .metric-headline {
    font-size: 1.35rem;
    line-height: 1.2;
    word-break: break-word;
  }
  .metric-fields {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 0.15rem 0.75rem;
    margin: 0;
    font-size: 0.85rem;
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
    width: 100%;
    max-width: 14rem;
    height: 0.4rem;
    background-color: rgb(from var(--muted) r g b / 0.35);
    border-radius: 999px;
    overflow: hidden;
  }
  .metric-bar span {
    display: block;
    height: 100%;
    background-color: var(--success);
  }
  .metric-bar[data-variant='warning'] span {
    background-color: var(--warning);
  }
  .metric-bar[data-variant='danger'] span {
    background-color: var(--danger);
  }
  .metric-pill {
    display: inline-block;
    margin: 0.1rem 0.2rem 0.1rem 0;
    padding: 0.05rem 0.4rem;
    background-color: rgb(from var(--muted) r g b / 0.35);
    border-radius: 0.25rem;
    font-size: 0.8rem;
  }
</style>
