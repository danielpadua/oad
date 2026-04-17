import * as React from "react"
import CodeMirror, { type Extension } from "@uiw/react-codemirror"
import { json, jsonParseLinter } from "@codemirror/lang-json"
import { linter, lintGutter } from "@codemirror/lint"
import { cn } from "@/lib/utils"

export interface JsonSchemaEditorProps {
  value: string
  onChange?: (value: string) => void
  readOnly?: boolean
  placeholder?: string
  minHeight?: string
  maxHeight?: string
  className?: string
  "aria-label"?: string
}

function buildExtensions(readOnly: boolean): Extension[] {
  const exts: Extension[] = [json()]
  if (!readOnly) {
    exts.push(lintGutter(), linter(jsonParseLinter()))
  }
  return exts
}

function JsonSchemaEditor({
  value,
  onChange,
  readOnly = false,
  placeholder = '{\n  \n}',
  minHeight = "200px",
  maxHeight = "500px",
  className,
  "aria-label": ariaLabel,
}: JsonSchemaEditorProps) {
  const extensions = React.useMemo(
    () => buildExtensions(readOnly),
    [readOnly]
  )

  return (
    <div
      className={cn(
        "overflow-hidden rounded-lg border border-border bg-card font-mono text-sm",
        readOnly && "opacity-90",
        className
      )}
      role="region"
      aria-label={ariaLabel ?? (readOnly ? "JSON Schema viewer" : "JSON Schema editor")}
    >
      <CodeMirror
        value={value}
        onChange={readOnly ? undefined : onChange}
        extensions={extensions}
        readOnly={readOnly}
        placeholder={placeholder}
        basicSetup={{
          lineNumbers: true,
          highlightActiveLineGutter: !readOnly,
          highlightActiveLine: !readOnly,
          foldGutter: true,
          autocompletion: !readOnly,
          bracketMatching: true,
          closeBrackets: !readOnly,
          indentOnInput: !readOnly,
        }}
        style={{ minHeight, maxHeight }}
        className="[&_.cm-editor]:outline-none [&_.cm-scroller]:overflow-auto"
        theme="none"
      />
    </div>
  )
}

export { JsonSchemaEditor }
