import AddIcon from "@mui/icons-material/Add";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import { Box, Button, IconButton, Tooltip, Typography } from "@mui/material";
import { useNavigate } from "react-router-dom";
import type { ComputeMachine } from "../types";
import { formatComputeTimestamp, formatStatus } from "../utils";
import { PageHeader, DataTable, StatusBadge } from "./primitives";
import type { Column, StatusVariant } from "./primitives";

type ComputeSectionProps = {
  loading: boolean;
  deletingMachineName: string;
  machines: ComputeMachine[];
  onDeleteMachine: (name: string) => void;
  onOpenCreate: () => void;
};

function statusOf(m: ComputeMachine, isDeleting: boolean): { v: StatusVariant; label: string; spin: boolean } {
  if (isDeleting) return { v: "error", label: "削除中", spin: true };
  if (!m.ready) return { v: "progress", label: formatStatus(m.status, "準備中"), spin: true };
  return { v: "ready", label: formatStatus(m.status, "実行中"), spin: false };
}

export function ComputeSection({
  loading, deletingMachineName, machines, onDeleteMachine, onOpenCreate
}: ComputeSectionProps) {
  const navigate = useNavigate();

  const columns: Column<ComputeMachine>[] = [
    {
      key: "name", header: "名前",
      render: (m) => (
        <Typography variant="body2" sx={{ fontWeight: 500, color: "primary.main" }}>
          {m.name}
        </Typography>
      )
    },
    {
      key: "status", header: "ステータス", width: 140,
      render: (m) => {
        const s = statusOf(m, deletingMachineName === m.name);
        return <StatusBadge variant={s.v} label={s.label} showSpinner={s.spin} />;
      }
    },
    {
      key: "updatedAt", header: "更新日時", width: 180,
      render: (m) => (
        <Typography variant="caption" color="text.secondary">
          {formatComputeTimestamp(m.updatedAt || m.createdAt)}
        </Typography>
      )
    },
    {
      key: "actions", header: "", width: 60, align: "right",
      render: (m) => (
        <Tooltip title="削除">
          <span>
            <IconButton
              size="small"
              color="error"
              disabled={deletingMachineName === m.name}
              onClick={() => onDeleteMachine(m.name)}
            >
              <DeleteOutlinedIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
      )
    }
  ];

  return (
    <Box>
      <PageHeader
        title="仮想マシン"
        actions={
          <Button variant="contained" startIcon={<AddIcon />} onClick={onOpenCreate}>
            作成
          </Button>
        }
      />
      <DataTable
        columns={columns}
        rows={machines}
        rowKey={(m) => m.name}
        onRowClick={(m) => navigate(`/compute/${encodeURIComponent(m.name)}`)}
        loading={loading}
        emptyMessage="まだ仮想マシンはありません"
      />
    </Box>
  );
}
