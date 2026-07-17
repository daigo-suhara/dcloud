import CloudQueueOutlinedIcon from "@mui/icons-material/CloudQueueOutlined";
import { Box, Typography } from "@mui/material";

type BrandProps = { compact?: boolean };

export function Brand({ compact = false }: BrandProps) {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1, minWidth: 0 }}>
      <CloudQueueOutlinedIcon sx={{ color: "primary.main", fontSize: compact ? 20 : 22 }} />
      <Typography sx={{ fontSize: 18, fontWeight: 500, letterSpacing: "-0.01em", color: "text.primary" }}>
        dcloud
      </Typography>
    </Box>
  );
}
