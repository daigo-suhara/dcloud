import { Box, Typography } from "@mui/material";
import type { ReactNode } from "react";

export type FormRowProps = {
  label: string;
  children: ReactNode;
};

// Left-label form row: label on the left column, input on the right.
// Direct child of an outlined Paper so consecutive rows share dividers.
export function FormRow({ label, children }: FormRowProps) {
  return (
    <Box
      sx={{
        display: "grid",
        gridTemplateColumns: { xs: "1fr", sm: "200px 1fr" },
        gap: { xs: 1, sm: 3 },
        px: { xs: 2, sm: 3 }, py: 2.5,
        borderTop: "1px solid",
        borderColor: "divider",
        "&:first-of-type": { borderTop: "none" },
        alignItems: "start"
      }}
    >
      <Typography variant="body2" sx={{ pt: 1, fontWeight: 500 }}>{label}</Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </Box>
  );
}
