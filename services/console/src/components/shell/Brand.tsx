import CloudQueueOutlinedIcon from "@mui/icons-material/CloudQueueOutlined";
import { Box, Typography } from "@mui/material";

type BrandProps = {
  compact?: boolean;
};

export function Brand({ compact = false }: BrandProps) {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1.25, minWidth: 0 }}>
      <CloudQueueOutlinedIcon sx={{ color: "primary.main", fontSize: compact ? 24 : 28 }} />
      <Typography variant="h6" sx={{ fontWeight: 700, whiteSpace: "nowrap", lineHeight: 1 }}>
        D Cloud
      </Typography>
    </Box>
  );
}

