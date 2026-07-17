import { Box, Typography } from "@mui/material";
import type { ReactNode } from "react";

export type PageHeaderProps = {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
};

// GCP-style page header: large title on the left, primary action(s) on
// the right. Sits above the section content and never inside a card.
export function PageHeader({ title, subtitle, actions }: PageHeaderProps) {
  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "flex-start",
        justifyContent: "space-between",
        gap: 2,
        pb: 2,
        mb: 2,
        borderBottom: "1px solid",
        borderColor: "divider"
      }}
    >
      <Box>
        <Typography variant="h4">{title}</Typography>
        {subtitle && (
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            {subtitle}
          </Typography>
        )}
      </Box>
      {actions && <Box sx={{ display: "flex", gap: 1 }}>{actions}</Box>}
    </Box>
  );
}
