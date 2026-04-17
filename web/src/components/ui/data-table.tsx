import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
  type VisibilityState,
} from "@tanstack/react-table"
import { ChevronDown, ChevronUp, ChevronsUpDown } from "lucide-react"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

export interface DataTablePaginationState {
  pageIndex: number
  pageSize: number
}

export interface DataTableProps<TData> {
  columns: ColumnDef<TData>[]
  data: TData[]
  isLoading?: boolean
  pagination?: DataTablePaginationState
  onPaginationChange?: (pagination: DataTablePaginationState) => void
  rowCount?: number
  sorting?: SortingState
  onSortingChange?: (sorting: SortingState) => void
  columnVisibility?: VisibilityState
  onColumnVisibilityChange?: (visibility: VisibilityState) => void
  emptyMessage?: string
  className?: string
}

function DataTableColumnHeader({
  column,
  title,
  className,
}: {
  column: { getIsSorted: () => false | "asc" | "desc"; toggleSorting: (desc?: boolean) => void; getCanSort: () => boolean }
  title: string
  className?: string
}) {
  if (!column.getCanSort()) {
    return <span className={cn("text-sm font-medium", className)}>{title}</span>
  }

  const sorted = column.getIsSorted()

  return (
    <button
      className={cn(
        "group/header inline-flex items-center gap-1.5 text-sm font-medium transition-colors hover:text-foreground",
        className
      )}
      onClick={() => column.toggleSorting(sorted === "asc")}
    >
      {title}
      {sorted === "asc" ? (
        <ChevronUp className="size-3.5 text-foreground" />
      ) : sorted === "desc" ? (
        <ChevronDown className="size-3.5 text-foreground" />
      ) : (
        <ChevronsUpDown className="size-3.5 opacity-40 group-hover/header:opacity-100" />
      )}
    </button>
  )
}

function DataTablePagination({
  pageIndex,
  pageSize,
  rowCount,
  onPaginationChange,
}: {
  pageIndex: number
  pageSize: number
  rowCount: number
  onPaginationChange: (p: DataTablePaginationState) => void
}) {
  const pageCount = Math.ceil(rowCount / pageSize)
  const canPrev = pageIndex > 0
  const canNext = pageIndex < pageCount - 1

  const start = pageIndex * pageSize + 1
  const end = Math.min((pageIndex + 1) * pageSize, rowCount)

  return (
    <div className="flex items-center justify-between px-1 py-2">
      <p className="text-muted-foreground text-sm">
        {rowCount === 0
          ? "No results"
          : `${start}–${end} of ${rowCount}`}
      </p>
      <div className="flex items-center gap-1.5">
        <Button
          variant="outline"
          size="sm"
          onClick={() => onPaginationChange({ pageIndex: 0, pageSize })}
          disabled={!canPrev}
        >
          «
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => onPaginationChange({ pageIndex: pageIndex - 1, pageSize })}
          disabled={!canPrev}
        >
          ‹
        </Button>
        <span className="text-muted-foreground text-sm tabular-nums">
          {pageIndex + 1} / {Math.max(pageCount, 1)}
        </span>
        <Button
          variant="outline"
          size="sm"
          onClick={() => onPaginationChange({ pageIndex: pageIndex + 1, pageSize })}
          disabled={!canNext}
        >
          ›
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => onPaginationChange({ pageIndex: pageCount - 1, pageSize })}
          disabled={!canNext}
        >
          »
        </Button>
      </div>
    </div>
  )
}

function DataTable<TData>({
  columns,
  data,
  isLoading = false,
  pagination,
  onPaginationChange,
  rowCount = 0,
  sorting,
  onSortingChange,
  columnVisibility,
  onColumnVisibilityChange,
  emptyMessage = "No results.",
  className,
}: DataTableProps<TData>) {
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: !!pagination,
    manualSorting: !!sorting,
    rowCount,
    state: {
      ...(pagination && { pagination }),
      ...(sorting && { sorting }),
      ...(columnVisibility && { columnVisibility }),
    },
    onPaginationChange: onPaginationChange
      ? (updater) => {
          const current = pagination ?? { pageIndex: 0, pageSize: 20 }
          const next =
            typeof updater === "function" ? updater(current) : updater
          onPaginationChange(next)
        }
      : undefined,
    onSortingChange: onSortingChange
      ? (updater) => {
          const current = sorting ?? []
          const next =
            typeof updater === "function" ? updater(current) : updater
          onSortingChange(next)
        }
      : undefined,
    onColumnVisibilityChange: onColumnVisibilityChange
      ? (updater) => {
          const current = columnVisibility ?? {}
          const next =
            typeof updater === "function" ? updater(current) : updater
          onColumnVisibilityChange(next)
        }
      : undefined,
  })

  const skeletonRows = pagination?.pageSize ?? 10

  return (
    <div className={cn("w-full space-y-2", className)}>
      <div className="rounded-lg border border-border overflow-hidden">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id} style={{ width: header.getSize() !== 150 ? header.getSize() : undefined }}>
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: skeletonRows }).map((_, i) => (
                <TableRow key={`skeleton-${i}`}>
                  {columns.map((_, j) => (
                    <TableCell key={j}>
                      <Skeleton className="h-4 w-full" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && "selected"}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-24 text-center text-muted-foreground">
                  {emptyMessage}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      {pagination && onPaginationChange && (
        <DataTablePagination
          pageIndex={pagination.pageIndex}
          pageSize={pagination.pageSize}
          rowCount={rowCount}
          onPaginationChange={onPaginationChange}
        />
      )}
    </div>
  )
}

export { DataTable, DataTableColumnHeader }
