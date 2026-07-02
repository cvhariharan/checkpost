export type SortDir = 'asc' | 'desc'
export type SortValue = string | number | null | undefined
export type SortState<C extends string> = { col: C; dir: SortDir } | null
export type SortAccessors<T, C extends string> = Record<C, (row: T) => SortValue>

function isMissing(v: SortValue): boolean {
  return v === null || v === undefined || v === ''
}

export function compareValues(a: SortValue, b: SortValue): number {
  const aMissing = isMissing(a)
  const bMissing = isMissing(b)
  if (aMissing && bMissing) return 0
  if (aMissing) return 1
  if (bMissing) return -1
  if (typeof a === 'number' && typeof b === 'number') return a - b
  return String(a).localeCompare(String(b), undefined, { numeric: true, sensitivity: 'base' })
}

export function sortRows<T, C extends string>(
  rows: readonly T[],
  state: SortState<C>,
  accessors: SortAccessors<T, C>,
  tiebreak?: (row: T) => SortValue
): T[] {
  if (!state) return [...rows]
  const { col, dir } = state
  const factor = dir === 'asc' ? 1 : -1
  const get = accessors[col]
  return [...rows].sort((a, b) => {
    const av = get(a)
    const bv = get(b)
    const primary = compareValues(av, bv)
    if (primary !== 0) {
      // Don't let direction flip the missing-last pinning.
      return isMissing(av) || isMissing(bv) ? primary : primary * factor
    }
    return tiebreak ? compareValues(tiebreak(a), tiebreak(b)) : 0
  })
}

// Toggle helper: same column flips direction, a new column starts ascending.
export function nextSort<C extends string>(state: SortState<C>, col: C): SortState<C> {
  if (state?.col === col) return { col, dir: state.dir === 'asc' ? 'desc' : 'asc' }
  return { col, dir: 'asc' }
}
