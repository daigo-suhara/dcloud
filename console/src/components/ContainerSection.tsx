import AddIcon from "@mui/icons-material/Add";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import CloudUploadOutlinedIcon from "@mui/icons-material/CloudUploadOutlined";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import ErrorOutlinedIcon from "@mui/icons-material/ErrorOutlined";
import GitHubIcon from "@mui/icons-material/GitHub";
import { alpha } from "@mui/material/styles";
import {
  Box, Button, Chip, CircularProgress, IconButton, Paper, TextField, Tooltip, Typography
} from "@mui/material";
import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Link as RouterLink, useNavigate } from "react-router-dom";
import type { DeployedService, UpdateForm } from "../types";
import { EnvVarEditor } from "./EnvVarEditor";
import { ContainerLogsViewer } from "./ContainerLogsViewer";
import { actionLinkButtonSx } from "../theme";
import { formatServiceStatus, formatServiceTimestamp, getServiceStatus } from "../utils";
import { PageHeader, DataTable, StatusBadge } from "./primitives";
import type { Column, StatusVariant } from "./primitives";

type ContainerSectionProps = {
  loading: boolean;
  deletingServiceName: string;
  updatingServiceName: string;
  onBackToList: () => void;
  onDeployClick: () => void;
  onDeleteService: (name: string) => void;
  onOpenService: (name: string) => void;
  onRepoConnectClick: () => void;
  onSetDomain: (name: string, domain: string) => Promise<void>;
  onUpdateService: (name: string, form: UpdateForm) => Promise<void>;
  selectedService: DeployedService | null;
  selectedStatus: ReturnType<typeof getServiceStatus> | null;
  containers: DeployedService[];
  activeProjectId: string;
};

function svcStatusVariant(status: ReturnType<typeof getServiceStatus> | null, isDeleting: boolean): { v: StatusVariant; spin: boolean } {
  if (isDeleting) return { v: "error", spin: true };
  if (status === "ready") return { v: "ready", spin: false };
  if (status === "loading") return { v: "progress", spin: true };
  return { v: "error", spin: false };
}

// ---- Detail (when a service is selected) --------------------------------
function ServiceDetail({
  service, status, updating, deleting,
  onBack, onUpdate, onDelete, onSetDomain, activeProjectId
}: {
  service: DeployedService;
  status: ReturnType<typeof getServiceStatus> | null;
  updating: boolean;
  deleting: boolean;
  onBack: () => void;
  onUpdate: (form: UpdateForm) => Promise<void>;
  onDelete: () => void;
  onSetDomain: (domain: string) => Promise<void>;
  activeProjectId: string;
}) {
  const [domainInput, setDomainInput] = useState("");
  const [savingDomain, setSavingDomain] = useState(false);
  const [form, setForm] = useState<UpdateForm>({
    image: service.image,
    port: String(service.port ?? 8080),
    minScale: String(service.minScale ?? 0),
    maxScale: String(service.maxScale ?? 20),
    startupScript: service.startupScript ?? "",
    env: service.env ?? []
  });

  useEffect(() => {
    setForm({
      image: service.image,
      port: String(service.port ?? 8080),
      minScale: String(service.minScale ?? 0),
      maxScale: String(service.maxScale ?? 20),
      startupScript: service.startupScript ?? "",
      env: service.env ?? []
    });
  }, [service.name]);

  const s = svcStatusVariant(status, deleting);

  return (
    <Box>
      <PageHeader
        title={service.name}
        actions={
          <Button startIcon={<ArrowBackIcon />} onClick={onBack}>一覧に戻る</Button>
        }
      />

      {/* Status summary */}
      <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" }, gap: 1, mb: 3 }}>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="caption" color="text.secondary">状態</Typography>
          <Box sx={{ mt: 0.5 }}>
            <StatusBadge variant={s.v} label={formatServiceStatus(service)} showSpinner={s.spin} />
          </Box>
        </Paper>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="caption" color="text.secondary">イメージ</Typography>
          <Typography variant="body2" sx={{ mt: 0.5, wordBreak: "break-all", fontFamily: "monospace" }}>
            {service.image}
          </Typography>
        </Paper>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="caption" color="text.secondary">公開 URL</Typography>
          <Box sx={{ mt: 0.5 }}>
            {service.url ? (
              <Button component="a" href={service.url} target="_blank" rel="noreferrer" variant="text" size="small" sx={actionLinkButtonSx}>
                {service.url}
              </Button>
            ) : <Typography variant="body2" color="text.secondary">-</Typography>}
          </Box>
        </Paper>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="caption" color="text.secondary">作成時刻</Typography>
          <Typography variant="body2" sx={{ mt: 0.5 }}>{service.createdAt ?? "-"}</Typography>
        </Paper>
      </Box>

      {/* Update form */}
      <Paper variant="outlined" sx={{ p: 2, mb: 3 }}>
        <Typography variant="h6" sx={{ mb: 2 }}>設定を更新</Typography>
        <Box
          component="form"
          onSubmit={async (e: FormEvent<HTMLFormElement>) => { e.preventDefault(); await onUpdate(form); }}
          sx={{ display: "grid", gap: 1.5 }}
        >
          <TextField
            label="コンテナイメージ" value={form.image}
            onChange={(e) => setForm(f => ({ ...f, image: e.target.value }))}
            disabled={updating} placeholder="ghcr.io/org/app:tag" fullWidth
            slotProps={{ htmlInput: { autoComplete: "off", autoCorrect: "off", autoCapitalize: "none", spellCheck: false } }}
          />
          <Box sx={{ display: "grid", gap: 1.5, gridTemplateColumns: { xs: "1fr", sm: "repeat(3, 1fr)" } }}>
            <TextField label="Port" type="number"
              slotProps={{ htmlInput: { min: 1, max: 65535 } }}
              value={form.port} onChange={(e) => setForm(f => ({ ...f, port: e.target.value }))} disabled={updating} />
            <TextField label="最小スケール" type="number"
              slotProps={{ htmlInput: { min: 0, max: 20 } }}
              value={form.minScale} onChange={(e) => setForm(f => ({ ...f, minScale: e.target.value }))} disabled={updating} />
            <TextField label="最大スケール" type="number"
              slotProps={{ htmlInput: { min: 1, max: 20 } }}
              value={form.maxScale} onChange={(e) => setForm(f => ({ ...f, maxScale: e.target.value }))} disabled={updating} />
          </Box>
          <EnvVarEditor value={form.env} onChange={(env) => setForm(f => ({ ...f, env }))} disabled={updating} size="small" />
          <TextField
            label="起動スクリプト（任意）" value={form.startupScript}
            onChange={(e) => setForm(f => ({ ...f, startupScript: e.target.value }))} disabled={updating}
            placeholder={"#!/bin/sh\nexec code-server --bind-addr 0.0.0.0:8080 --auth none ."}
            multiline minRows={3} fullWidth
            slotProps={{ htmlInput: { autoComplete: "off", autoCorrect: "off", autoCapitalize: "none", spellCheck: false, style: { fontFamily: "monospace", fontSize: "0.85rem" } } }}
          />
          <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
            <Button
              type="submit" variant="contained"
              disabled={updating || !form.image.trim()}
              startIcon={updating ? <CircularProgress size={14} sx={{ color: "inherit" }} /> : undefined}
            >
              {updating ? "更新中..." : "更新"}
            </Button>
          </Box>
        </Box>
      </Paper>

      {/* Custom domain */}
      <Paper variant="outlined" sx={{ p: 2, mb: 3 }}>
        <Typography variant="h6" sx={{ mb: 2 }}>カスタムドメイン</Typography>
        {service.customDomain ? (
          <Box sx={{ display: "grid", gap: 1 }}>
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
              <Button component="a" href={`https://${service.customDomain}`} target="_blank" rel="noreferrer" variant="text" size="small" sx={{ ...actionLinkButtonSx, fontWeight: 500 }}>
                {service.customDomain}
              </Button>
              <Button
                variant="outlined" color="error" size="small" disabled={savingDomain}
                onClick={async () => { setSavingDomain(true); try { await onSetDomain(""); } finally { setSavingDomain(false); } }}
              >
                {savingDomain ? <CircularProgress size={14} /> : "削除"}
              </Button>
            </Box>
            {service.domainStatus === "ready" && (
              <Chip label="有効" color="success" size="small" icon={<CheckCircleIcon />} sx={{ width: "fit-content" }} />
            )}
            {service.domainStatus === "pending" && (
              <Tooltip title={service.domainStatusReason ?? "DNS または TLS の設定を待機中"}>
                <Chip label="DNS 待機中" size="small"
                  icon={<CircularProgress size={12} thickness={5} sx={{ color: "inherit !important" }} />}
                  sx={{ width: "fit-content", bgcolor: alpha("#f59e0b", 0.12), color: "warning.dark", "& .MuiChip-icon": { color: "warning.dark" } }} />
              </Tooltip>
            )}
            {service.domainStatus === "error" && (
              <Tooltip title={service.domainStatusReason ?? ""}>
                <Chip label="エラー" color="error" size="small" icon={<ErrorOutlinedIcon />} sx={{ width: "fit-content" }} />
              </Tooltip>
            )}
            {service.domainStatus === "pending" && service.domainCnameTarget && (
              <Typography variant="caption" color="text.secondary">
                CNAME レコードを <Box component="code" sx={{ bgcolor: "grey.100", px: 0.5, borderRadius: 0.5 }}>{service.domainCnameTarget}</Box> に向けてください
              </Typography>
            )}
          </Box>
        ) : (
          <Box sx={{ display: "flex", gap: 1, alignItems: "flex-start" }}>
            <TextField size="small" placeholder="example.com" value={domainInput}
              onChange={(e) => setDomainInput(e.target.value)} disabled={savingDomain} sx={{ flex: 1 }} />
            <Button
              variant="contained" size="small"
              disabled={savingDomain || !domainInput.trim()}
              onClick={async () => { setSavingDomain(true); try { await onSetDomain(domainInput); setDomainInput(""); } finally { setSavingDomain(false); } }}
              sx={{ whiteSpace: "nowrap" }}
            >
              {savingDomain ? <CircularProgress size={14} /> : "設定"}
            </Button>
          </Box>
        )}
      </Paper>

      <ContainerLogsViewer serviceName={service.name} projectId={activeProjectId} enabled={service.ready} />

      <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 3 }}>
        <Button
          variant="outlined" color="error"
          startIcon={deleting ? <CircularProgress size={14} sx={{ color: "inherit" }} /> : <DeleteOutlinedIcon />}
          onClick={onDelete} disabled={deleting}
        >
          {deleting ? "削除中..." : "サービスを削除"}
        </Button>
      </Box>
    </Box>
  );
}

// ---- List (default) -----------------------------------------------------
export function ContainerSection({
  loading, deletingServiceName, updatingServiceName,
  onBackToList, onDeployClick, onDeleteService, onOpenService,
  onRepoConnectClick, onSetDomain, onUpdateService,
  selectedService, selectedStatus, containers, activeProjectId
}: ContainerSectionProps) {
  const navigate = useNavigate();

  if (selectedService) {
    return (
      <ServiceDetail
        service={selectedService}
        status={selectedStatus}
        updating={updatingServiceName === selectedService.name}
        deleting={deletingServiceName === selectedService.name}
        onBack={onBackToList}
        onUpdate={(form) => onUpdateService(selectedService.name, form)}
        onDelete={() => onDeleteService(selectedService.name)}
        onSetDomain={(domain) => onSetDomain(selectedService.name, domain)}
        activeProjectId={activeProjectId}
      />
    );
  }

  const columns: Column<DeployedService>[] = [
    {
      key: "name", header: "名前",
      render: (svc) => (
        <Typography variant="body2" sx={{ fontWeight: 500, color: "primary.main" }}>{svc.name}</Typography>
      )
    },
    {
      key: "status", header: "ステータス", width: 140,
      render: (svc) => {
        const s = svcStatusVariant(getServiceStatus(svc), deletingServiceName === svc.name);
        return <StatusBadge variant={s.v} label={formatServiceStatus(svc)} showSpinner={s.spin} />;
      }
    },
    {
      key: "updatedAt", header: "更新日時", width: 180,
      render: (svc) => (
        <Typography variant="caption" color="text.secondary">
          {svc.updatedAt || svc.createdAt ? formatServiceTimestamp(svc.updatedAt || svc.createdAt || "") : "-"}
        </Typography>
      )
    },
    {
      key: "actions", header: "", width: 60, align: "right",
      render: (svc) => (
        <Tooltip title="削除">
          <span>
            <IconButton size="small" color="error"
              disabled={deletingServiceName === svc.name}
              onClick={() => onDeleteService(svc.name)}>
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
        title="コンテナ"
        actions={
          <>
            <Button
              component={RouterLink} to="/container/repository"
              variant="outlined" startIcon={<GitHubIcon />}
              onClick={onRepoConnectClick}
            >
              リポジトリを接続
            </Button>
            <Button
              component={RouterLink} to="/container/deploy"
              variant="contained" startIcon={<AddIcon />}
              onClick={onDeployClick}
            >
              デプロイ
            </Button>
          </>
        }
      />
      <DataTable
        columns={columns}
        rows={containers}
        rowKey={(svc) => svc.name}
        onRowClick={(svc) => { onOpenService(svc.name); navigate(`/container/${encodeURIComponent(svc.name)}`); }}
        loading={loading}
        emptyMessage="まだサービスはありません"
      />
    </Box>
  );
}

// keep CloudUploadOutlinedIcon import valid
void CloudUploadOutlinedIcon;
