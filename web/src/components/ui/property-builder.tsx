import { useState } from "react"
import { Plus, Trash2, ChevronDown, ChevronUp, Code, LayoutList } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { JsonSchemaEditor } from "@/components/ui/json-schema-editor"

// ─── Types ────────────────────────────────────────────────────────────────────

type PropertyType = "string" | "number" | "integer" | "boolean" | "array" | "object"
type ArrayItemType = "string" | "number" | "integer" | "boolean"

interface PropertyEntry {
  id: string
  name: string
  type: PropertyType
  description: string
  required: boolean
  // string constraints
  minLength: string
  maxLength: string
  pattern: string
  // number/integer constraints
  minimum: string
  maximum: string
  // array constraint
  itemsType: ArrayItemType | ""
}

// ─── Conversion ───────────────────────────────────────────────────────────────

function newEntry(): PropertyEntry {
  return {
    id: crypto.randomUUID(),
    name: "",
    type: "string",
    description: "",
    required: false,
    minLength: "",
    maxLength: "",
    pattern: "",
    minimum: "",
    maximum: "",
    itemsType: "",
  }
}

function entriesToSchema(entries: PropertyEntry[]): string {
  const properties: Record<string, unknown> = {}
  const required: string[] = []

  for (const entry of entries) {
    const key = entry.name.trim()
    if (!key) continue

    const prop: Record<string, unknown> = { type: entry.type }
    if (entry.description.trim()) prop.description = entry.description.trim()

    if (entry.type === "string") {
      if (entry.minLength !== "") prop.minLength = Number(entry.minLength)
      if (entry.maxLength !== "") prop.maxLength = Number(entry.maxLength)
      if (entry.pattern.trim()) prop.pattern = entry.pattern.trim()
    }
    if (entry.type === "number" || entry.type === "integer") {
      if (entry.minimum !== "") prop.minimum = Number(entry.minimum)
      if (entry.maximum !== "") prop.maximum = Number(entry.maximum)
    }
    if (entry.type === "array" && entry.itemsType) {
      prop.items = { type: entry.itemsType }
    }

    properties[key] = prop
    if (entry.required) required.push(key)
  }

  const schema: Record<string, unknown> = { type: "object", properties }
  if (required.length > 0) schema.required = required
  return JSON.stringify(schema, null, 2)
}

function schemaToEntries(jsonStr: string): PropertyEntry[] {
  try {
    const schema = JSON.parse(jsonStr) as Record<string, unknown>
    if (!schema || typeof schema !== "object" || !schema.properties) return []
    const required = (schema.required as string[]) ?? []
    const props = schema.properties as Record<string, Record<string, unknown>>

    return Object.entries(props).map(([name, def]) => ({
      id: crypto.randomUUID(),
      name,
      type: (def.type as PropertyType) ?? "string",
      description: (def.description as string) ?? "",
      required: required.includes(name),
      minLength: def.minLength != null ? String(def.minLength) : "",
      maxLength: def.maxLength != null ? String(def.maxLength) : "",
      pattern: (def.pattern as string) ?? "",
      minimum: def.minimum != null ? String(def.minimum) : "",
      maximum: def.maximum != null ? String(def.maximum) : "",
      itemsType:
        ((def.items as Record<string, unknown> | undefined)
          ?.type as ArrayItemType) ?? "",
    }))
  } catch {
    return []
  }
}

// ─── Component ────────────────────────────────────────────────────────────────

interface PropertyBuilderProps {
  value: string
  onChange?: (value: string) => void
  readOnly?: boolean
}

export function PropertyBuilder({ value, onChange, readOnly = false }: PropertyBuilderProps) {
  const [mode, setMode] = useState<"visual" | "raw">("visual")
  const [entries, setEntries] = useState<PropertyEntry[]>(() =>
    schemaToEntries(value)
  )
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set())

  function commit(next: PropertyEntry[]) {
    setEntries(next)
    onChange?.(entriesToSchema(next))
  }

  function addEntry() {
    const entry = newEntry()
    commit([...entries, entry])
    setExpandedIds((prev) => new Set([...prev, entry.id]))
  }

  function removeEntry(id: string) {
    commit(entries.filter((e) => e.id !== id))
    setExpandedIds((prev) => {
      const next = new Set(prev)
      next.delete(id)
      return next
    })
  }

  function patch(id: string, partial: Partial<PropertyEntry>) {
    commit(entries.map((e) => (e.id === id ? { ...e, ...partial } : e)))
  }

  function toggleExpand(id: string) {
    setExpandedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function switchToRaw() {
    setMode("raw")
  }

  function switchToVisual() {
    setEntries(schemaToEntries(value))
    setExpandedIds(new Set())
    setMode("visual")
  }

  if (mode === "raw") {
    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <p className="text-xs text-muted-foreground">
            {readOnly ? "Raw JSON Schema." : "Editing raw JSON Schema — advanced use only."}
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
          minHeight="260px"
          aria-label="Allowed properties JSON Schema"
        />
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs text-muted-foreground">
          {entries.length === 0
            ? "No properties defined."
            : `${entries.length} ${entries.length === 1 ? "property" : "properties"} defined.`}
        </p>
        <Button variant="ghost" size="sm" onClick={switchToRaw}>
          <Code className="mr-1.5 size-3.5" />
          Raw JSON
        </Button>
      </div>

      {entries.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border py-8 text-center text-sm text-muted-foreground">
          {readOnly ? "No properties defined." : "No properties yet. Click \"Add Property\" to define the first one."}
        </div>
      ) : (
        <div className="space-y-2">
          {entries.map((entry) => {
            const expanded = expandedIds.has(entry.id)
            return (
              <div
                key={entry.id}
                className="rounded-lg border border-border bg-card"
              >
                {/* Summary row */}
                <div className="flex items-center gap-2 p-2">
                  <Input
                    placeholder="property_name"
                    value={entry.name}
                    readOnly={readOnly}
                    onChange={(e) => patch(entry.id, { name: e.target.value })}
                    className="h-8 min-w-0 flex-1 font-mono text-sm"
                  />
                  <Select
                    value={entry.type}
                    onValueChange={(v) =>
                      !readOnly && patch(entry.id, { type: v as PropertyType })
                    }
                    disabled={readOnly}
                  >
                    <SelectTrigger className="h-8 w-28 shrink-0">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="string">string</SelectItem>
                      <SelectItem value="number">number</SelectItem>
                      <SelectItem value="integer">integer</SelectItem>
                      <SelectItem value="boolean">boolean</SelectItem>
                      <SelectItem value="array">array</SelectItem>
                      <SelectItem value="object">object</SelectItem>
                    </SelectContent>
                  </Select>
                  <div className="flex shrink-0 items-center gap-1.5">
                    <Checkbox
                      id={`req-${entry.id}`}
                      checked={entry.required}
                      disabled={readOnly}
                      onCheckedChange={(c) =>
                        !readOnly && patch(entry.id, { required: !!c })
                      }
                    />
                    <Label
                      htmlFor={`req-${entry.id}`}
                      className="text-xs text-muted-foreground"
                    >
                      required
                    </Label>
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 w-8 shrink-0 p-0"
                    onClick={() => toggleExpand(entry.id)}
                    aria-label={expanded ? "Collapse" : "Expand constraints"}
                  >
                    {expanded ? (
                      <ChevronUp className="size-3.5" />
                    ) : (
                      <ChevronDown className="size-3.5" />
                    )}
                  </Button>
                  {!readOnly && (
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-8 w-8 shrink-0 p-0"
                      onClick={() => removeEntry(entry.id)}
                      aria-label="Remove property"
                    >
                      <Trash2 className="size-3.5 text-destructive" />
                    </Button>
                  )}
                </div>

                {/* Constraints panel */}
                {expanded && (
                  <div className="space-y-3 border-t border-border px-3 py-3">
                    {entry.description || !readOnly ? (
                      <div className="space-y-1.5">
                        <Label className="text-xs">Description</Label>
                        <Input
                          placeholder="Optional description for this property"
                          value={entry.description}
                          readOnly={readOnly}
                          onChange={(e) =>
                            patch(entry.id, { description: e.target.value })
                          }
                          className="h-8 text-sm"
                        />
                      </div>
                    ) : null}

                    {entry.type === "string" && (
                      <div className="grid grid-cols-3 gap-2">
                        <div className="space-y-1.5">
                          <Label className="text-xs">Min Length</Label>
                          <Input
                            type="number"
                            min={0}
                            placeholder="0"
                            value={entry.minLength}
                            readOnly={readOnly}
                            onChange={(e) =>
                              patch(entry.id, { minLength: e.target.value })
                            }
                            className="h-8 text-sm"
                          />
                        </div>
                        <div className="space-y-1.5">
                          <Label className="text-xs">Max Length</Label>
                          <Input
                            type="number"
                            min={0}
                            placeholder="∞"
                            value={entry.maxLength}
                            readOnly={readOnly}
                            onChange={(e) =>
                              patch(entry.id, { maxLength: e.target.value })
                            }
                            className="h-8 text-sm"
                          />
                        </div>
                        <div className="space-y-1.5">
                          <Label className="text-xs">Pattern (regex)</Label>
                          <Input
                            placeholder="e.g. ^[a-z]+$"
                            value={entry.pattern}
                            readOnly={readOnly}
                            onChange={(e) =>
                              patch(entry.id, { pattern: e.target.value })
                            }
                            className="h-8 font-mono text-sm"
                          />
                        </div>
                      </div>
                    )}

                    {(entry.type === "number" || entry.type === "integer") && (
                      <div className="grid grid-cols-2 gap-2">
                        <div className="space-y-1.5">
                          <Label className="text-xs">Minimum</Label>
                          <Input
                            type="number"
                            placeholder="-∞"
                            value={entry.minimum}
                            readOnly={readOnly}
                            onChange={(e) =>
                              patch(entry.id, { minimum: e.target.value })
                            }
                            className="h-8 text-sm"
                          />
                        </div>
                        <div className="space-y-1.5">
                          <Label className="text-xs">Maximum</Label>
                          <Input
                            type="number"
                            placeholder="∞"
                            value={entry.maximum}
                            readOnly={readOnly}
                            onChange={(e) =>
                              patch(entry.id, { maximum: e.target.value })
                            }
                            className="h-8 text-sm"
                          />
                        </div>
                      </div>
                    )}

                    {entry.type === "array" && (
                      <div className="space-y-1.5">
                        <Label className="text-xs">Items Type</Label>
                        <Select
                          value={entry.itemsType || "string"}
                          onValueChange={(v) =>
                            !readOnly && patch(entry.id, { itemsType: v as ArrayItemType })
                          }
                          disabled={readOnly}
                        >
                          <SelectTrigger className="h-8 w-36">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="string">string</SelectItem>
                            <SelectItem value="number">number</SelectItem>
                            <SelectItem value="integer">integer</SelectItem>
                            <SelectItem value="boolean">boolean</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                    )}

                    {(entry.type === "boolean" || entry.type === "object") && (
                      <p className="text-xs text-muted-foreground">
                        No additional constraints for this type.
                      </p>
                    )}
                  </div>
                )}
              </div>
            )
          })}
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
          Add Property
        </Button>
      )}
    </div>
  )
}
