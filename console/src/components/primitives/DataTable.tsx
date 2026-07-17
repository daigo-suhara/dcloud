import {
  Box, Checkbox, Paper, Table, TableBody, TableCell, TableContainer,
  TableHead, TableRow, Typography
} from "@mui/material";
import type { ReactNode } from "react";

export type Column<T> = {
  key: string;
  header: string;
  width?: number | string;
  align?: "left" | "right" | "center";
  render: (row: T) => ReactNode;
};

export type DataTableProps<T> = {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  onRowClick?: (row: T) => void;
  selectable?: boolean;
  selectedKeys?: Set<string>;
  onToggleSelect?: (key: string) => void;
  onToggleSelectAll?: () => void;
  emptyMessage?: string;
  loading?: boolean;
};

// GCP-style dense table with optional checkbox selection and row-hover
// row actions. Row cells stay tight; put action buttons in the last
// column with `align: "right"`.
export function DataTable<T>({
  columns, rows, rowKey, onRowClick,
  selectable, selectedKeys, onToggleSelect, onToggleSelectAll,
  emptyMessage = "データがありません", loading
}: DataTableProps<T>) {
  const allSelected = selectable && rows.length > 0 && selectedKeys?.size === rows.length;
  const someSelected = selectable && (selectedKeys?.size ?? 0) > 0 && !allSelected;

  return (
    <TableContainer component={Paper} variant="outlined" sx={{ borderRadius: 1 }}>
      <Table size="small">
        <TableHead>
          <TableRow>
            {selectable && (
              <TableCell padding="checkbox" sx={{ width: 42 }}>
                <Checkbox
                  size="small"
                  indeterminate={someSelected}
                  checked={allSelected}
                  onChange={onToggleSelectAll}
                />
              </TableCell>
            )}
            {columns.map(c => (
              <TableCell key={c.key} align={c.align ?? "left"} sx={{ width: c.width }}>
                {c.header}
              </TableCell>
            ))}
          </TableRow>
        </TableHead>
        <TableBody>
          {loading ? (
            <TableRow>
              <TableCell colSpan={columns.length + (selectable ? 1 : 0)}>
                <Typography variant="body2" color="text.secondary" align="center" sx={{ py: 4 }}>
                  読み込み中...
                </Typography>
              </TableCell>
            </TableRow>
          ) : rows.length === 0 ? (
            <TableRow>
              <TableCell colSpan={columns.length + (selectable ? 1 : 0)}>
                <Box sx={{ py: 5, textAlign: "center" }}>
                  <Typography variant="body2" color="text.secondary">{emptyMessage}</Typography>
                </Box>
              </TableCell>
            </TableRow>
          ) : (
            rows.map(row => {
              const k = rowKey(row);
              return (
                <TableRow
                  key={k}
                  hover={!!onRowClick}
                  sx={{ cursor: onRowClick ? "pointer" : "default" }}
                >
                  {selectable && (
                    <TableCell padding="checkbox" onClick={(e) => e.stopPropagation()}>
                      <Checkbox
                        size="small"
                        checked={selectedKeys?.has(k) ?? false}
                        onChange={() => onToggleSelect?.(k)}
                      />
                    </TableCell>
                  )}
                  {columns.map(c => (
                    <TableCell
                      key={c.key}
                      align={c.align ?? "left"}
                      onClick={c.key === "actions" ? (e) => e.stopPropagation() : (onRowClick ? () => onRowClick(row) : undefined)}
                    >
                      {c.render(row)}
                    </TableCell>
                  ))}
                </TableRow>
              );
            })
          )}
        </TableBody>
      </Table>
    </TableContainer>
  );
}
