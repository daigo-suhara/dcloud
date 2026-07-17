import CloseIcon from "@mui/icons-material/Close";
import { Box, Divider, Drawer, IconButton, List, ListItemButton, ListItemIcon, ListItemText } from "@mui/material";
import { BiHomeAlt2 } from "react-icons/bi";
import { BsDatabase, BsHdd } from "react-icons/bs";
import { PiCpuBold, PiShippingContainer } from "react-icons/pi";
import { navItems, type RouteState } from "../../types";
import { Brand } from "./Brand";

type DrawerNavProps = {
  onCloseSidebar: () => void;
  onNavigate: (section: RouteState["section"]) => void;
  route: RouteState;
  sidebarOpen: boolean;
};

const ICONS: Record<string, React.ElementType> = {
  home: BiHomeAlt2,
  container: PiShippingContainer,
  compute: PiCpuBold,
  storage: BsHdd,
  database: BsDatabase,
};

function isSelected(route: RouteState, id: string) {
  if (route.section === id) return true;
  if (id === "database" && route.section === "database-detail") return true;
  if (id === "compute" && route.section === "compute-create") return true;
  if (id === "container" && (route.section === "deploy" || route.section === "repository")) return true;
  return false;
}

export function DrawerNav({ onCloseSidebar, onNavigate, route, sidebarOpen }: DrawerNavProps) {
  return (
    <Drawer
      open={sidebarOpen}
      onClose={onCloseSidebar}
      variant="temporary"
      ModalProps={{ keepMounted: true }}
      slotProps={{
        paper: {
          sx: { width: 280, borderRight: 1, borderColor: "divider" }
        }
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, minHeight: 56, px: 2, borderBottom: 1, borderColor: "divider" }}>
        <IconButton onClick={onCloseSidebar} size="small" aria-label="close navigation">
          <CloseIcon />
        </IconButton>
        <Brand />
      </Box>

      <List sx={{ py: 1 }}>
        {navItems.map((item) => {
          const selected = isSelected(route, item.id);
          const Icon = ICONS[item.id] ?? BiHomeAlt2;
          return (
            <ListItemButton
              key={item.id}
              selected={selected}
              onClick={() => {
                onNavigate(item.id);
                if (window.matchMedia("(max-width: 760px)").matches) onCloseSidebar();
              }}
              sx={{
                mx: 1, borderRadius: 24, minHeight: 40, pl: 2,
                "&.Mui-selected": {
                  bgcolor: "rgba(26,115,232,0.08)",
                  color: "primary.main",
                  "&:hover": { bgcolor: "rgba(26,115,232,0.12)" }
                }
              }}
            >
              <ListItemIcon sx={{ minWidth: 32, color: selected ? "primary.main" : "text.secondary" }}>
                <Box component={Icon} sx={{ fontSize: 20 }} />
              </ListItemIcon>
              <ListItemText
                primary={item.label}
                slotProps={{ primary: { sx: { fontSize: 14, fontWeight: selected ? 500 : 400 } } }}
              />
            </ListItemButton>
          );
        })}
      </List>
      <Divider sx={{ mt: "auto" }} />
    </Drawer>
  );
}
