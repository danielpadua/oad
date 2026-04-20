import { useState } from "react"
import { Plus, Trash2, Code, LayoutList } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { JsonSchemaEditor } from "@/components/ui/json-schema-editor"

// ─── Types ────────────────────────────────────────────────────────────────────

interface RelationEntry {
  id: string
  name: string
  targetTypes: string // comma-separated entity type names
}

// ─── Conversion ───────────────────────────────────────────────────────────────

function newEntry(): RelationEntry {
  return { id: crypto.randomUUID(), name: "", targetTypes: "" }
}

function entriesToSchema(entries: RelationEntry[]): string {
  const result: Record<string, { target_types: string[] }> = {}
  for (const entry of entries) {
    const key = entry.name.trim()
    if (!key) continue
    result[key] = {
      target_types: entry.targetTypes
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
    }
  }
  return JSON.stringify(result, null, 2)
}

function schemaToEntries(jsonStr: string): RelationEntry[] {
  try {
    const schema = JSON.parse(jsonStr) as Record<
      string,
      { target_types?: string[] }
    >
    if (!schema || typeof schema !== "object") return []
    return Object.entries(schema).map(([name, def]) => ({
      id: crypto.randomUUID(),
      name,
      targetTypes: (def.target_types ?? []).join(", "),
    }))
  } catch {
    return []
  }
}

// ─── Component ────────────────────────────────────────────────────────────────

interface RelationBuilderProps {
  value: string
  onChange?: (value: string) => void
  readOnly?: boolean
}

export function RelationBuilder({ value, onChange, readOnly = false }: RelationBuilderProps) {
  const [mode, setMode] = useState<"visual" | "raw">("visual")
  const [entries, setEntries] = useState<RelationEntry[]>(() =>
    schemaToEntries(value)
  )

  function commit(next: RelationEntry[]) {
    setEntries(next)
    onChange?.(entriesToSchema(next))
  }

  function addEntry() {
    commit([...entries, newEntry()])
  }

  function removeEntry(id: string) {
    commit(entries.filter((e) => e.id !== id))
  }

  function patch(id: string, partial: Partial<RelationEntry>) {
    commit(entries.map((e) => (e.id === id ? { ...e, ...partial } : e)))
  }

  function switchToRaw() {
    setMode("raw")
  }

  function switchToVisual() {
    setEntries(schemaToEntries(value))
    setMode("visual")
  }

  if (mode === "raw") {
    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <p className="text-xs text-muted-foreground">
            {readOnly ? "Raw JSON." : "Editing raw JSON — advanced use only."}
          </p>
          <Button variant="ghost" size="sm" onClick={switchToVisual}>
            <LayoutList className="mr-1.5 size-3.5" />
            Visual editor
          </Button>
        </div>
        <JsonSchemaEditor
          value={value}
          onChange={readOnly ? undefined : onChange}
          readOnly={readOnly}
          minHeight="200px"
          placeholder="{}"
          aria-label="Allowed relations"
        />
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs text-muted-foreground">
          {entries.length === 0
            ? "No relations defined."
            : `${entries.length} ${entries.length === 1 ? "relation" : "relations"} defined.`}
        </p>
        <Button variant="ghost" size="sm" onClick={switchToRaw}>
          <Code className="mr-1.5 size-3.5" />
          Raw JSON
        </Button>
      </div>

      {entries.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border py-8 text-center text-sm text-muted-foreground">
          {readOnly ? "No relations defined." : "No relations yet. Click \"Add Relation\" to define the first one."}
        </div>
      ) : (
        <div className="space-y-2">
          {/* Column headers */}
          <div className={`grid items-center gap-2 px-1 ${readOnly ? "grid-cols-[1fr_1.6fr]" : "grid-cols-[1fr_1.6fr_auto]"}`}>
            <Label className="text-xs text-muted-foreground">
              Relation name
            </Label>
            <Label className="text-xs text-muted-foreground">
              Target entity types
              <span className="ml-1 font-normal opacity-60">
                (comma-separated)
              </span>
            </Label>
            {!readOnly && <span className="w-8" />}
          </div>

          {entries.map((entry) => (
            <div
              key={entry.id}
              className={`grid items-center gap-2 ${readOnly ? "grid-cols-[1fr_1.6fr]" : "grid-cols-[1fr_1.6fr_auto]"}`}
            >
              <Input
                placeholder="e.g. member, owns"
                value={entry.name}
                readOnly={readOnly}
                onChange={(e) => patch(entry.id, { name: e.target.value })}
                className="h-8 font-mono text-sm"
              />
              <Input
                placeholder="e.g. user, group, role"
                value={entry.targetTypes}
                readOnly={readOnly}
                onChange={(e) =>
                  patch(entry.id, { targetTypes: e.target.value })
                }
                className="h-8 text-sm"
              />
              {!readOnly && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-8 w-8 shrink-0 p-0"
                  onClick={() => removeEntry(entry.id)}
                  aria-label="Remove relation"
                >
                  <Trash2 className="size-3.5 text-destructive" />
                </Button>
              )}
            </div>
          ))}
        </div>
      )}

      {!readOnly && (
        <Button
          variant="outline"
          size="sm"
          onClick={addEntry}
          className="w-full"
        >
          <Plus className="mr-1.5 size-3.5" />
          Add Relation
        </Button>
      )}
    </div>
  )
}
