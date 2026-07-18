import AddIcon from "@mui/icons-material/Add";
import DeleteIcon from "@mui/icons-material/Delete";
import { Box, Button, IconButton, MenuItem, TextField, Typography } from "@mui/material";
import { useEffect, useState } from "react";
import type { Bucket, BucketVolumeEntry } from "../types";

type Props = {
  value: BucketVolumeEntry[];
  onChange: (v: BucketVolumeEntry[]) => void;
  projectId: string;
  disabled?: boolean;
};

export function BucketVolumesEditor({ value, onChange, projectId, disabled }: Props) {
  const [buckets, setBuckets] = useState<Bucket[]>([]);

  useEffect(() => {
    fetch("/api/v1/storage", { credentials: "include", headers: { "X-DCP-Project": projectId } })
      .then(r => r.json())
      .then(d => setBuckets(d.buckets ?? []))
      .catch(() => setBuckets([]));
  }, [projectId]);

  function update(index: number, patch: Partial<BucketVolumeEntry>) {
    onChange(value.map((v, i) => (i === index ? { ...v, ...patch } : v)));
  }
  function remove(index: number) {
    onChange(value.filter((_, i) => i !== index));
  }
  function add() {
    onChange([...value, { bucketName: buckets[0]?.name ?? "", mountPath: "" }]);
  }

  const noBuckets = buckets.length === 0;

  return (
    <Box sx={{ display: "grid", gap: 1 }}>
      {noBuckets && value.length === 0 && (
        <Typography variant="caption" color="text.secondary">
          バケットがまだありません。ストレージ画面で作成してください。
        </Typography>
      )}
      {value.map((entry, i) => (
        <Box key={i} sx={{ display: "grid", gridTemplateColumns: "1fr 1fr auto", gap: 1, alignItems: "center" }}>
          <TextField
            select size="small" label="バケット" value={entry.bucketName}
            onChange={(e) => update(i, { bucketName: e.target.value })}
            disabled={disabled || buckets.length === 0}
          >
            {buckets.map(b => <MenuItem key={b.name} value={b.name}>{b.name}</MenuItem>)}
          </TextField>
          <TextField
            size="small" label="マウントパス" value={entry.mountPath}
            onChange={(e) => update(i, { mountPath: e.target.value })}
            disabled={disabled}
            placeholder="/var/www/html/wp-content"
          />
          <IconButton size="small" onClick={() => remove(i)} disabled={disabled} color="error">
            <DeleteIcon fontSize="small" />
          </IconButton>
        </Box>
      ))}
      <Box>
        <Button size="small" startIcon={<AddIcon />} onClick={add} disabled={disabled || noBuckets} variant="outlined">
          追加
        </Button>
      </Box>
    </Box>
  );
}
