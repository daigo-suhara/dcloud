import {
  Button, CircularProgress, Dialog, DialogActions, DialogContent, DialogTitle,
  Typography
} from "@mui/material";
import type { ReactNode } from "react";

export type FormDialogProps = {
  open: boolean;
  title: string;
  onClose: () => void;
  onSubmit: () => void | Promise<void>;
  submitLabel?: string;
  submitting?: boolean;
  submitDisabled?: boolean;
  error?: string;
  children: ReactNode;
  maxWidth?: "xs" | "sm" | "md";
};

// Standard create/edit dialog. Header + body + footer with Cancel /
// primary action. Handles submitting state via disabled + spinner.
export function FormDialog({
  open, title, onClose, onSubmit,
  submitLabel = "作成", submitting, submitDisabled,
  error, children, maxWidth = "xs"
}: FormDialogProps) {
  return (
    <Dialog open={open} onClose={() => !submitting && onClose()} fullWidth maxWidth={maxWidth}>
      <DialogTitle sx={{ fontSize: 18, fontWeight: 500 }}>{title}</DialogTitle>
      <DialogContent sx={{ display: "grid", gap: 2, pt: "8px !important" }}>
        {error && <Typography color="error" variant="body2">{error}</Typography>}
        {children}
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={onClose} disabled={submitting}>キャンセル</Button>
        <Button
          variant="contained"
          onClick={() => void onSubmit()}
          disabled={submitting || submitDisabled}
        >
          {submitting ? <CircularProgress size={16} /> : submitLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
