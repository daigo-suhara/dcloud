import CloseIcon from "@mui/icons-material/Close";
import { Box, Divider, Drawer, IconButton, List, ListItemButton, ListItemIcon, ListItemText } from "@mui/material";
import { alpha } from "@mui/material/styles";
import { BiHomeAlt2, BiServer } from "react-icons/bi";
import { BsDatabase, BsHdd } from "react-icons/bs";
import { PiCpuBold } from "react-icons/pi";
import { PiShippingContainer } from "react-icons/pi";
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
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, minHeight: 64, px: 2 }}>
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
                borderRadius: 1.5,
                minHeight: 48,
                "&.Mui-selected": {
                  bgcolor: alpha("#2563eb", 0.08)
                }
              }}
            >
              <ListItemIcon sx={{ minWidth: 40 }}>
                <Box sx={{ width: 28, height: 28, borderRadius: "999px", display: "grid", placeItems: "center", bgcolor: route.section === item.id ? alpha("#2563eb", 0.12) : alpha("#0f172a", 0.04), color: route.section === item.id ? "primary.main" : "text.secondary" }}>
                  {item.id === "home" ? (
                    <Box component={BiHomeAlt2} sx={{ fontSize: 18 }} />
                  ) : item.id === "container" ? (
                    <Box component={PiShippingContainer} sx={{ fontSize: 18 }} />
                  ) : item.id === "compute" ? (
                    <Box component={PiCpuBold} sx={{ fontSize: 18 }} />
                  ) : item.id === "storage" ? (
                    <Box component={BsHdd} sx={{ fontSize: 18 }} />
                  ) : item.id === "database" ? (
                    <Box component={BsDatabase} sx={{ fontSize: 18 }} />
                  ) : (
                    <Box component={BiServer} sx={{ fontSize: 18 }} />
                  )}
                </Box>
              </ListItemIcon>
              <ListItemText primary={<Box component="span" sx={{ fontWeight: 600 }}>{item.label}</Box>} sx={{ my: 0 }} />
            </ListItemButton>
          ))}
        </List>
      </Box>
    </Drawer>
  );
}
