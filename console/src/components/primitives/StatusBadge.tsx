import { Box, CircularProgress, Typography } from "@mui/material";

export type StatusVariant = "ready" | "progress" | "error" | "unknown";

const COLORS: Record<StatusVariant, { fg: string; dot: string }> = {
  ready:    { fg: "#137333", dot: "#137333" },
  progress: { fg: "#1a73e8", dot: "#1a73e8" },
  error:    { fg: "#c5221f", dot: "#c5221f" },
  unknown:  { fg: "#5f6368", dot: "#9aa0a6" }
};

export type StatusBadgeProps = {
  variant: StatusVariant;
  label: string;
  showSpinner?: boolean;
};

export function StatusBadge({ variant, label, showSpinner }: StatusBadgeProps) {
  const c = COLORS[variant];
  return (
    <Box sx={{ display: "inline-flex", alignItems: "center", gap: 0.75 }}>
      {showSpinner
        ? <CircularProgress size={10} thickness={6} sx={{ color: c.dot }} />
        : <Box sx={{ width: 8, height: 8, borderRadius: "50%", bgcolor: c.dot }} />}
      <Typography variant="caption" sx={{ color: c.fg, fontWeight: 500 }}>
        {label}
      </Typography>
    </Box>
  );
}
