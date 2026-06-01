import CloseIcon from "@mui/icons-material/Close";
import CloudUploadOutlinedIcon from "@mui/icons-material/CloudUploadOutlined";
import HomeOutlinedIcon from "@mui/icons-material/HomeOutlined";
import StorageOutlinedIcon from "@mui/icons-material/StorageOutlined";
import { Box, Divider, Drawer, IconButton, List, ListItemButton, ListItemIcon, ListItemText, Typography } from "@mui/material";
import { alpha } from "@mui/material/styles";
import { navItems, type RouteState } from "../../types";
import { Brand } from "./Brand";

type DrawerNavProps = {
  onCloseSidebar: () => void;
  onNavigate: (section: RouteState["section"]) => void;
  route: RouteState;
  sidebarOpen: boolean;
};

export function DrawerNav({ onCloseSidebar, onNavigate, route, sidebarOpen }: DrawerNavProps) {
  return (
    <Drawer
      open={sidebarOpen}
      onClose={onCloseSidebar}
      variant="temporary"
      ModalProps={{ keepMounted: true }}
      slotProps={{
        paper: {
          sx: {
            width: { xs: "88vw", sm: 300 },
            maxWidth: 300,
            borderRadius: "0 16px 16px 0",
            m: 0,
            border: "1px solid rgba(148, 163, 184, 0.24)",
            boxShadow: "0 24px 48px rgba(15, 23, 42, 0.12)"
          }
        }
      }}
    >
      <Box sx={{ display: "flex", flexDirection: "column", gap: 0, pt: 0, pb: 2 }}>
        <Box sx={{ display: "flex", alignItems: "center", minHeight: 64, px: 2 }}>
          <IconButton onClick={onCloseSidebar} aria-label="close navigation" sx={{ width: 40, height: 40, p: 0, flex: "0 0 auto" }}>
            <CloseIcon />
          </IconButton>
          <Box sx={{ flex: "1 1 auto", minWidth: 0, display: "flex", alignItems: "center" }}>
            <Brand />
          </Box>
        </Box>
        <Divider sx={{ mt: 0, mb: 1.5 }} />
        <List disablePadding>
          {navItems.map((item) => (
            <ListItemButton
              key={item.id}
              selected={route.section === item.id}
              onClick={() => {
                onNavigate(item.id);
                if (window.matchMedia("(max-width: 760px)").matches) {
                  onCloseSidebar();
                }
              }}
              sx={{
                mb: 1,
                borderRadius: 1.5,
                border: "1px solid rgba(148, 163, 184, 0.24)",
                minHeight: 48,
                "&.Mui-selected": {
                  bgcolor: alpha("#2563eb", 0.08),
                  borderColor: alpha("#2563eb", 0.28)
                }
              }}
            >
              <ListItemIcon sx={{ minWidth: 40 }}>
                {item.id === "home" ? <HomeOutlinedIcon /> : item.id === "container" ? <StorageOutlinedIcon /> : <CloudUploadOutlinedIcon />}
              </ListItemIcon>
              <ListItemText primary={item.label} sx={{ my: 0 }} />
            </ListItemButton>
          ))}
        </List>
      </Box>
    </Drawer>
  );
}
