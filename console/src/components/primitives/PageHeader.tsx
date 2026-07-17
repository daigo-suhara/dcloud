import { Box, Typography } from "@mui/material";
import type { ReactNode } from "react";

export type PageHeaderProps = {
  title: string;
  actions?: ReactNode;
};

// Large title on the left, primary action(s) on the right. Sits above
// the section content and never inside a card.
export function PageHeader({ title, actions }: PageHeaderProps) {
  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 2,
        pb: 2,
        mb: 2,
        borderBottom: "1px solid",
        borderColor: "divider"
      }}
    >
      <Typography variant="h4">{title}</Typography>
      {actions && <Box sx={{ display: "flex", gap: 1 }}>{actions}</Box>}
    </Box>
  );
}
