// CodeMirror SQL editor for SqlEditor.svelte. Separate module so CodeMirror
// code-splits into its own lazy chunk. SQLite dialect + osquery autocompletion.
import { EditorView, keymap, placeholder as placeholderExt, lineNumbers as lineNumbersExt } from '@codemirror/view'
import { Compartment, type Extension } from '@codemirror/state'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { sql, SQLite, type SQLNamespace } from '@codemirror/lang-sql'
import {
  autocompletion,
  completionKeymap,
  closeBrackets,
  closeBracketsKeymap
} from '@codemirror/autocomplete'
import { syntaxHighlighting, HighlightStyle, bracketMatching, indentOnInput } from '@codemirror/language'
import { tags as t } from '@lezer/highlight'
import type { OsquerySchema } from './api'

// Shape the osquery schema into @codemirror/lang-sql's SQLNamespace: each table
// completes (with description/platforms) and its columns complete beneath it.
function buildSqlSchema(schema: OsquerySchema): SQLNamespace {
  const ns: Record<string, SQLNamespace> = {}
  for (const table of schema.tables) {
    ns[table.name] = {
      self: { label: table.name, type: 'type', detail: table.platforms.join(', '), info: table.description },
      children: table.columns.map((col) => ({
        label: col.name,
        type: 'property',
        detail: col.type,
        info: col.description
      }))
    }
  }
  return ns
}

// Syntax colors mapped to oat theme tokens; CSS vars follow light/dark.
const oatHighlight = HighlightStyle.define([
  { tag: t.keyword, color: 'var(--primary)', fontWeight: '600' },
  { tag: [t.string, t.special(t.string)], color: 'var(--success)' },
  { tag: [t.number, t.bool, t.null], color: 'var(--warning)' },
  { tag: t.comment, color: 'var(--muted-foreground)', fontStyle: 'italic' },
  { tag: [t.operator, t.punctuation, t.separator], color: 'var(--muted-foreground)' },
  { tag: [t.function(t.variableName), t.function(t.propertyName)], color: 'var(--foreground)' },
  { tag: [t.propertyName, t.variableName, t.name], color: 'var(--foreground)' }
])

const MONO = "var(--font-mono, ui-monospace, 'JetBrains Mono', 'SFMono-Regular', Menlo, monospace)"

// Editor chrome themed from oat CSS variables.
const oatTheme = EditorView.theme({
  '&': {
    color: 'var(--foreground)',
    backgroundColor: 'var(--card)',
    border: '1px solid var(--input)',
    borderRadius: 'var(--radius, 6px)',
    fontSize: '0.8125rem'
  },
  '&.cm-focused': {
    outline: 'none',
    borderColor: 'var(--ring)',
    boxShadow: '0 0 0 1px var(--ring)'
  },
  '.cm-content': {
    fontFamily: MONO,
    padding: '8px 0',
    caretColor: 'var(--foreground)'
  },
  '.cm-gutters': {
    backgroundColor: 'transparent',
    color: 'var(--muted-foreground)',
    border: 'none'
  },
  '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--foreground)' },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection': {
    backgroundColor: 'color-mix(in srgb, var(--primary) 22%, transparent)'
  },
  '.cm-placeholder': { color: 'var(--faint-foreground)' },
  '.cm-tooltip': {
    backgroundColor: 'var(--card)',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius, 6px)',
    color: 'var(--foreground)'
  },
  '.cm-tooltip-autocomplete > ul': { fontFamily: MONO },
  '.cm-tooltip-autocomplete > ul > li[aria-selected]': {
    backgroundColor: 'var(--primary)',
    color: 'var(--primary-foreground)'
  },
  '.cm-completionDetail': { color: 'var(--muted-foreground)', fontStyle: 'normal' },
  '.cm-completionInfo': {
    backgroundColor: 'var(--card)',
    border: '1px solid var(--border)',
    color: 'var(--muted-foreground)'
  }
})

export type SqlEditorOptions = {
  parent: HTMLElement
  doc: string
  placeholder?: string
  lineNumbers?: boolean
  disabled?: boolean
  minHeight?: string
  maxHeight?: string
  onChange: (value: string) => void
  onSubmit?: () => void
}

export type SqlEditorHandle = {
  setSchema: (schema: OsquerySchema) => void
  setDisabled: (disabled: boolean) => void
  getValue: () => string
  setValue: (value: string) => void
  destroy: () => void
}

export function createSqlEditor(opts: SqlEditorOptions): SqlEditorHandle {
  const langCompartment = new Compartment()
  const editableCompartment = new Compartment()

  const buildLang = (schema?: SQLNamespace) =>
    sql({ dialect: SQLite, upperCaseKeywords: true, schema })

  const extensions: Extension[] = [
    // Mod-Enter submits; before defaultKeymap so it beats its blank-line binding.
    keymap.of([
      { key: 'Mod-Enter', preventDefault: true, run: () => { opts.onSubmit?.(); return true } }
    ]),
    langCompartment.of(buildLang()),
    autocompletion(),
    history(),
    closeBrackets(),
    bracketMatching(),
    indentOnInput(),
    syntaxHighlighting(oatHighlight),
    keymap.of([...closeBracketsKeymap, ...completionKeymap, ...historyKeymap, ...defaultKeymap]),
    oatTheme,
    EditorView.theme({
      '.cm-content': { minHeight: opts.minHeight ?? '6em' },
      '.cm-scroller': { maxHeight: opts.maxHeight ?? '30em', overflowY: 'auto' }
    }),
    EditorView.lineWrapping,
    editableCompartment.of(EditorView.editable.of(!opts.disabled)),
    EditorView.updateListener.of((u) => {
      if (u.docChanged) opts.onChange(u.state.doc.toString())
    })
  ]
  if (opts.placeholder) extensions.push(placeholderExt(opts.placeholder))
  if (opts.lineNumbers) extensions.unshift(lineNumbersExt())

  const view = new EditorView({ parent: opts.parent, doc: opts.doc, extensions })

  return {
    setSchema(schema) {
      view.dispatch({ effects: langCompartment.reconfigure(buildLang(buildSqlSchema(schema))) })
    },
    setDisabled(disabled) {
      view.dispatch({ effects: editableCompartment.reconfigure(EditorView.editable.of(!disabled)) })
    },
    getValue: () => view.state.doc.toString(),
    setValue(value) {
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } })
    },
    destroy: () => view.destroy()
  }
}
