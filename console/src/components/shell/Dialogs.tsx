import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import { Alert, Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography } from "@mui/material";

type DialogsProps = {
  deletingMachineName: string;
  deletingName: string;
  deletingProjectId: string;
  onCancelDelete: () => void;
  onCancelDeleteMachine: () => void;
  onCancelProjectDelete: () => void;
  onConfirmDelete: (name: string) => void;
  onConfirmDeleteMachine: (name: string) => void;
  onConfirmDeleteProject: (projectId: string) => void;
  pendingDeleteMachineName: string;
  pendingDeleteName: string;
  pendingProjectDeleteId: string;
  pendingProjectDeleteName: string;
};

export function Dialogs({
  deletingMachineName,
  deletingName,
  deletingProjectId,
  onCancelDelete,
  onCancelDeleteMachine,
  onCancelProjectDelete,
  onConfirmDelete,
  onConfirmDeleteMachine,
  onConfirmDeleteProject,
  pendingDeleteMachineName,
  pendingDeleteName,
  pendingProjectDeleteId,
  pendingProjectDeleteName
}: DialogsProps) {
  return (
    <>
      <Dialog open={Boolean(pendingDeleteName)} onClose={onCancelDelete} fullWidth maxWidth="sm">
        <DialogTitle sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <DeleteOutlinedIcon color="error" />
          削除の確認
        </DialogTitle>
        <DialogContent dividers>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <Typography variant="h6">
              {pendingDeleteName}
            </Typography>
            <Typography color="text.secondary">このサービスを削除しますか？</Typography>
          </Box>
        </DialogContent>
        <DialogActions sx={{ p: 2 }}>
          <Button onClick={onCancelDelete} variant="outlined">
            キャンセル
          </Button>
          <Button onClick={() => onConfirmDelete(pendingDeleteName)} variant="contained" color="error" disabled={deletingName === pendingDeleteName}>
            {deletingName === pendingDeleteName ? "削除中..." : "削除"}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={Boolean(pendingDeleteMachineName)} onClose={onCancelDeleteMachine} fullWidth maxWidth="sm">
        <DialogTitle sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <DeleteOutlinedIcon color="error" />
          削除の確認
        </DialogTitle>
        <DialogContent dividers>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <Typography variant="h6">
              {pendingDeleteMachineName}
            </Typography>
            <Typography color="text.secondary">この仮想マシンを削除しますか？</Typography>
          </Box>
        </DialogContent>
        <DialogActions sx={{ p: 2 }}>
          <Button onClick={onCancelDeleteMachine} variant="outlined">
            キャンセル
          </Button>
          <Button onClick={() => onConfirmDeleteMachine(pendingDeleteMachineName)} variant="contained" color="error" disabled={deletingMachineName === pendingDeleteMachineName}>
            {deletingMachineName === pendingDeleteMachineName ? "削除中..." : "削除"}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={Boolean(pendingProjectDeleteId)} onClose={onCancelProjectDelete} fullWidth maxWidth="sm">
        <DialogTitle sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <DeleteOutlinedIcon color="error" />
          削除の確認
        </DialogTitle>
        <DialogContent dividers>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <Typography variant="h6">
              {pendingProjectDeleteName}
            </Typography>
            <Typography color="text.secondary">このプロジェクトを削除しますか？</Typography>
          </Box>
        </DialogContent>
        <DialogActions sx={{ p: 2 }}>
          <Button onClick={onCancelProjectDelete} variant="outlined">
            キャンセル
          </Button>
          <Button onClick={() => onConfirmDeleteProject(pendingProjectDeleteId)} variant="contained" color="error" disabled={deletingProjectId === pendingProjectDeleteId}>
            {deletingProjectId === pendingProjectDeleteId ? "削除中..." : "削除"}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
