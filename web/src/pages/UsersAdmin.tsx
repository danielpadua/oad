import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import type { ColumnDef } from "@tanstack/react-table"
import { useTranslation } from "react-i18next"
import { Search } from "lucide-react"

import { http } from "@/lib/http-client"
import type { HttpError } from "@/lib/http-client"
import type { PaginatedResponse, UserAdmin } from "@/lib/types"
import { FadeContent } from "@/components/reactbits"
import { DataTable, DataTableColumnHeader } from "@/components/ui/data-table"
import type { DataTablePaginationState } from "@/components/ui/data-table"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"

function useUsers(pagination: DataTablePaginationState) {
  const params = new URLSearchParams({
    limit: String(pagination.pageSize),
    offset: String(pagination.pageIndex * pagination.pageSize),
  })

  return useQuery<PaginatedResponse<UserAdmin>, HttpError>({
    queryKey: ["users-admin", pagination],
    queryFn: () => http.get<PaginatedResponse<UserAdmin>>(`/api/v1/users?${params.toString()}`),
  })
}

function buildColumns(t: (key: string) => string): ColumnDef<UserAdmin>[] {
  return [
    {
      accessorKey: "user",
      header: t("usersAdmin.columns.user"),
      cell: ({ row }) => {
        const props = row.original.properties || {}
        const userName = (props.userName as string) || row.original.external_id
        const displayName = props.displayName as string
        return (
          <div className="flex flex-col">
            <span className="font-medium text-foreground">{displayName || userName}</span>
            {displayName && <span className="text-xs text-muted-foreground">{userName}</span>}
          </div>
        )
      },
    },
    {
      accessorKey: "email",
      header: t("usersAdmin.columns.email"),
      cell: ({ row }) => {
        const props = row.original.properties || {}
        const emails = props.emails as Array<{ value: string }>
        const primaryEmail = emails?.[0]?.value || (props.email as string) || "—"
        return <span className="text-sm text-muted-foreground">{primaryEmail}</span>
      },
    },
    {
      accessorKey: "status",
      header: t("usersAdmin.columns.status"),
      cell: ({ row }) => {
        const props = row.original.properties || {}
        const active = props.active !== false // assume active by default unless explicitly false
        if (active) {
          return <Badge variant="secondary" className="bg-green-500/10 text-green-600 hover:bg-green-500/20">{t("usersAdmin.status.active")}</Badge>
        }
        return <Badge variant="destructive" className="bg-red-500/10 text-red-600 hover:bg-red-500/20">{t("usersAdmin.status.disabled")}</Badge>
      },
    },
    {
      accessorKey: "identities",
      header: t("usersAdmin.columns.identities"),
      cell: ({ row }) => {
        const identities = row.original.external_identities || []
        if (identities.length === 0) {
          return <span className="text-xs text-muted-foreground">—</span>
        }
        return (
          <div className="flex flex-wrap gap-1">
            {identities.map((id, idx) => (
              <Badge key={idx} variant="outline" title={id.external_subject} className="font-mono text-xs">
                {id.provider_name}
              </Badge>
            ))}
          </div>
        )
      },
    },
    {
      accessorKey: "created_at",
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t("usersAdmin.columns.createdAt")} />
      ),
      cell: ({ row }) => new Date(row.original.created_at).toLocaleDateString(),
    },
  ]
}

export default function UsersAdmin() {
  const { t } = useTranslation()
  const [search, setSearch] = useState("")
  const [pagination, setPagination] = useState<DataTablePaginationState>({
    pageIndex: 0,
    pageSize: 25,
  })

  const { data, isLoading } = useUsers(pagination)

  const allItems = data?.items ?? []
  
  // Local filtering by username/displayName/email. 
  // In a real app with large data, this should be a backend search.
  const filtered = search.trim()
    ? allItems.filter((u) => {
        const term = search.toLowerCase()
        const props = u.properties || {}
        const userName = (props.userName as string || "").toLowerCase()
        const displayName = (props.displayName as string || "").toLowerCase()
        const email = (props.email as string || "").toLowerCase()
        const emails = (props.emails as Array<{ value: string }> | undefined)
        const hasEmail = emails?.some(e => e.value.toLowerCase().includes(term))
        
        return (
          u.external_id.toLowerCase().includes(term) ||
          userName.includes(term) ||
          displayName.includes(term) ||
          email.includes(term) ||
          hasEmail
        )
      })
    : allItems

  const columns = buildColumns(t)

  return (
    <FadeContent duration={0.4} className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">{t("usersAdmin.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("usersAdmin.subtitle")}
          </p>
        </div>
      </div>

      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-xs">
          <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder={t("usersAdmin.searchPlaceholder")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8"
          />
        </div>

        {!isLoading && (
          <span className="text-sm text-muted-foreground">
            {data?.total ?? 0} {(data?.total ?? 0) === 1 ? t("usersAdmin.userCountSingle") : t("usersAdmin.userCountPlural")}
          </span>
        )}
      </div>

      <DataTable
        columns={columns}
        data={filtered}
        isLoading={isLoading}
        pagination={pagination}
        onPaginationChange={setPagination}
        rowCount={data?.total ?? 0}
        emptyMessage={t("usersAdmin.emptyMessage")}
      />
    </FadeContent>
  )
}
