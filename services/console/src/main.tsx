import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  Alert,
  AppBar,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Container,
  CssBaseline,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  Drawer,
  IconButton,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  MenuItem,
  Paper,
  Snackbar,
  TextField,
  Toolbar,
  Typography,
  createTheme,
  ThemeProvider
} from "@mui/material";
import { alpha } from "@mui/material/styles";
import AddOutlinedIcon from "@mui/icons-material/AddOutlined";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import CloseIcon from "@mui/icons-material/Close";
import CloudQueueOutlinedIcon from "@mui/icons-material/CloudQueueOutlined";
import CloudUploadOutlinedIcon from "@mui/icons-material/CloudUploadOutlined";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import ErrorOutlinedIcon from "@mui/icons-material/ErrorOutlined";
import GitHubIcon from "@mui/icons-material/GitHub";
import HomeOutlinedIcon from "@mui/icons-material/HomeOutlined";
import HourglassTopIcon from "@mui/icons-material/HourglassTop";
import LogoutIcon from "@mui/icons-material/Logout";
import MenuIcon from "@mui/icons-material/Menu";
import StorageOutlinedIcon from "@mui/icons-material/StorageOutlined";
import "./styles.css";

type PlatformResponse = {
  namespace: string;
  user: string;
  projectId: string;
  services: DeployedService[];
};

type ProjectsResponse = {
  user: string;
  projects: Project[];
  defaultProjectId: string;
};

type AuthUser = {
  id: string;
  username: string;
  email?: string;
  name?: string;
};

type Project = {
  id: string;
  name: string;
  owner: string;
  createdAt: string;
};

type DeployedService = {
  name: string;
  image: string;
  url?: string;
  ready: boolean;
  reason?: string;
  createdAt?: string;
  updatedAt?: string;
  namespace: string;
  projectId?: string;
  generation?: number;
};

type DeployForm = {
  name: string;
  image: string;
  port: string;
  minScale: string;
  maxScale: string;
};

const theme = createTheme({
  palette: {
    mode: "light",
    primary: { main: "#0f172a" },
    background: {
      default: "#f8fafc",
      paper: "#ffffff"
    },
    text: {
      primary: "#0f172a",
      secondary: "#64748b"
    }
  },
  shape: {
    borderRadius: 16
  },
  typography: {
    fontFamily: '"Noto Sans JP", sans-serif',
    button: {
      textTransform: "none",
      fontWeight: 600
    }
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        body: {
          margin: 0
        },
        a: {
          color: "inherit",
          textDecoration: "none"
        }
      }
    },
    MuiButton: {
      defaultProps: {
        disableElevation: true
      },
      styleOverrides: {
        root: {
          borderRadius: 12
        }
      }
    },
    MuiPaper: {
      styleOverrides: {
        root: {
          backgroundImage: "none"
        }
      }
    }
  }
});

const initialForm: DeployForm = {
  name: "",
  image: "",
  port: "",
  minScale: "0",
  maxScale: "1"
};

const navItems = [
  { id: "home", label: "ホーム" },
  { id: "container", label: "コンテナ" },
  { id: "deploy", label: "仮想マシン" }
] as const;

type RouteState = {
  section: (typeof navItems)[number]["id"];
  selectedServiceName: string | null;
};

function parseRoute(hash: string): RouteState {
  const route = hash.replace(/^#/, "");

  if (!route) {
    return { section: "home", selectedServiceName: null };
  }

  const [section, ...rest] = route.split("/");
  const normalizedSection = section === "services" ? "container" : section;

  if (normalizedSection === "container" && rest.length > 0) {
    return {
      section: "container",
      selectedServiceName: decodeURIComponent(rest.join("/"))
    };
  }

  if (navItems.some((item) => item.id === normalizedSection)) {
    return { section: normalizedSection as RouteState["section"], selectedServiceName: null };
  }

  return { section: "home", selectedServiceName: null };
}

const shellBg = {
  background:
    "radial-gradient(circle at top left, rgba(37, 99, 235, 0.08), transparent 24%), radial-gradient(circle at top right, rgba(15, 23, 42, 0.05), transparent 26%), linear-gradient(180deg, #ffffff 0%, #f8fafc 45%, #eef2f7 100%)"
} as const;

function App() {
  const [currentUser, setCurrentUser] = useState<AuthUser | null>(null);
  const [services, setServices] = useState<DeployedService[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeProjectId, setActiveProjectId] = useState("");
  const [projectName, setProjectName] = useState("");
  const [creatingProject, setCreatingProject] = useState(false);
  const [showProjectCreateForm, setShowProjectCreateForm] = useState(false);
  const [deletingProjectId, setDeletingProjectId] = useState("");
  const [pendingProjectDeleteId, setPendingProjectDeleteId] = useState("");
  const [authLoading, setAuthLoading] = useState(true);
  const [route, setRoute] = useState<RouteState>(() => parseRoute(window.location.hash));
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [deletingName, setDeletingName] = useState("");
  const [pendingDeleteName, setPendingDeleteName] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [form, setForm] = useState(initialForm);

  const selectedService =
    route.section === "container" && route.selectedServiceName
      ? services.find((service) => service.name === route.selectedServiceName)
      : null;
  const selectedStatus = selectedService ? getServiceStatus(selectedService) : null;

  useEffect(() => {
    if (!message) {
      return;
    }
    const timer = window.setTimeout(() => setMessage(""), 3500);
    return () => window.clearTimeout(timer);
  }, [message]);

  useEffect(() => {
    const handleHashChange = () => setRoute(parseRoute(window.location.hash));
    window.addEventListener("hashchange", handleHashChange);
    void loadCurrentUser();
    return () => window.removeEventListener("hashchange", handleHashChange);
  }, []);

  useEffect(() => {
    if (!currentUser) {
      setProjects([]);
      setServices([]);
      setActiveProjectId("");
      setProjectName("");
      return;
    }

    setProjects([]);
    setServices([]);
    setActiveProjectId("");
    const savedProject = localStorage.getItem(projectStorageKey(currentUser.id));
    if (savedProject) {
      setActiveProjectId(savedProject);
    }
    void loadProjects();
  }, [currentUser]);

  useEffect(() => {
    if (!activeProjectId) {
      return;
    }

    void loadServices();
    const timer = window.setInterval(() => {
      void loadServices({ quiet: true });
    }, 5000);

    return () => window.clearInterval(timer);
  }, [activeProjectId]);

  function apiHeaders(extra?: HeadersInit) {
    const headers = new Headers(extra);
    if (activeProjectId) {
      headers.set("X-DCP-Project", activeProjectId);
    }
    return headers;
  }

  async function loadCurrentUser() {
    setAuthLoading(true);
    try {
      const response = await fetch("/api/v1/auth/me", {
        credentials: "include"
      });
      if (response.status === 401) {
        setCurrentUser(null);
        return;
      }
      const data = (await response.json()) as AuthUser | { error?: string };
      if (!response.ok) {
        throw new Error("error" in data && data.error ? data.error : "ログイン状態を確認できませんでした");
      }
      if ("id" in data) {
        setCurrentUser(data);
      }
    } catch (authError) {
      setError(authError instanceof Error ? authError.message : "ログイン状態を確認できませんでした");
    } finally {
      setAuthLoading(false);
    }
  }

  function startLogin() {
    window.location.href = "/api/v1/auth/login";
  }

  function startRegister() {
    window.location.href = "/api/v1/auth/register";
  }

  function startLogout() {
    window.location.href = "/api/v1/auth/logout";
  }

  function projectStorageKey(userId: string) {
    return `dcp-active-project:${userId}`;
  }

  function handleProjectSelect(projectId: string) {
    setActiveProjectId(projectId);
    if (currentUser) {
      localStorage.setItem(projectStorageKey(currentUser.id), projectId);
    }
  }

  async function loadProjects() {
    if (!currentUser) {
      return;
    }
    try {
      const response = await fetch("/api/v1/projects", {
        credentials: "include",
        headers: apiHeaders()
      });
      const data = (await response.json()) as ProjectsResponse | { error?: string };
      if (!response.ok) {
        throw new Error("error" in data && data.error ? data.error : "プロジェクト一覧を読み込めませんでした");
      }
      if ("projects" in data) {
        setProjects(data.projects);
        const saved = localStorage.getItem(projectStorageKey(currentUser.id));
        const nextProject = data.projects.find((project) => project.id === saved)?.id ?? data.defaultProjectId;
        handleProjectSelect(nextProject);
      }
    } catch (projectError) {
      setError(projectError instanceof Error ? projectError.message : "プロジェクト一覧を読み込めませんでした");
    }
  }

  async function loadServices(options?: { quiet?: boolean }) {
    if (!activeProjectId || !currentUser) {
      return;
    }
    if (!options?.quiet) {
      setLoading(true);
      setError("");
    }
    try {
      const response = await fetch("/api/v1/services", {
        credentials: "include",
        headers: apiHeaders()
      });
      const data = (await response.json()) as PlatformResponse | { error?: string };
      if (!response.ok) {
        throw new Error("error" in data && data.error ? data.error : "サービス一覧を読み込めませんでした");
      }
      if ("namespace" in data) {
        setServices(data.services ?? []);
      }
    } catch (loadError) {
      if (!options?.quiet) {
        setError(loadError instanceof Error ? loadError.message : "サービス一覧を読み込めませんでした");
      }
    } finally {
      if (!options?.quiet) {
        setLoading(false);
      }
    }
  }

  async function handleCreateProject(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setCreatingProject(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch("/api/v1/projects", {
        credentials: "include",
        method: "POST",
        headers: apiHeaders({
          "Content-Type": "application/json"
        }),
        body: JSON.stringify({ name: projectName.trim() })
      });
      const data = (await response.json()) as Project | { error?: string };
      if (!response.ok) {
        throw new Error("error" in data && data.error ? data.error : "プロジェクトの作成に失敗しました");
      }
      if ("id" in data) {
        setProjects((current) => [...current, data]);
        handleProjectSelect(data.id);
        setProjectName("");
        setShowProjectCreateForm(false);
        setMessage(`${data.name} を作成しました`);
      }
    } catch (projectError) {
      setError(projectError instanceof Error ? projectError.message : "プロジェクトの作成に失敗しました");
    } finally {
      setCreatingProject(false);
    }
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch("/api/v1/services", {
        method: "POST",
        credentials: "include",
        headers: apiHeaders({
          "Content-Type": "application/json"
        }),
        body: JSON.stringify({
          name: form.name.trim(),
          image: form.image.trim(),
          port: Number(form.port || "8080"),
          minScale: Number(form.minScale || "0"),
          maxScale: Number(form.maxScale || "1")
        })
      });

      const data = (await response.json()) as DeployedService | { error?: string };
      if (!response.ok) {
        throw new Error("error" in data && data.error ? data.error : "サービスの作成に失敗しました");
      }
      if ("name" in data) {
        setMessage(`${data.name} を作成しました`);
      }
      setForm((current) => ({ ...current, name: "hello-dcp", minScale: "0", maxScale: "1" }));
      await loadServices();
    } catch (deployError) {
      setError(deployError instanceof Error ? deployError.message : "サービスの作成に失敗しました");
    } finally {
      setSubmitting(false);
    }
  }

  function requestDelete(name: string) {
    setPendingDeleteName(name);
  }

  function cancelDelete() {
    setPendingDeleteName("");
  }

  function requestDeleteProject(projectId: string) {
    setPendingProjectDeleteId(projectId);
  }

  function cancelProjectDelete() {
    setPendingProjectDeleteId("");
  }

  async function confirmDelete(name: string) {
    setPendingDeleteName("");
    setDeletingName(name);
    setError("");
    setMessage("");
    try {
      const response = await fetch(`/api/v1/services/${name}`, {
        method: "DELETE",
        credentials: "include",
        headers: apiHeaders()
      });
      if (!response.ok && response.status !== 204) {
        const data = (await response.json()) as { error?: string };
        throw new Error(data.error ?? "サービスの削除に失敗しました");
      }
      setMessage(`${name} を削除しました`);
      if (route.selectedServiceName === name) {
        window.location.hash = "#container";
      }
      await loadServices();
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : "サービスの削除に失敗しました");
    } finally {
      setDeletingName("");
    }
  }

  async function confirmDeleteProject(projectId: string) {
    setPendingProjectDeleteId("");
    setDeletingProjectId(projectId);
    setError("");
    setMessage("");
    try {
      const response = await fetch(`/api/v1/projects/${projectId}`, {
        method: "DELETE",
        credentials: "include",
        headers: apiHeaders()
      });
      if (!response.ok && response.status !== 204) {
        const data = (await response.json()) as { error?: string };
        throw new Error(data.error ?? "プロジェクトの削除に失敗しました");
      }
      setMessage("プロジェクトを削除しました");
      await loadProjects();
    } catch (projectError) {
      setError(projectError instanceof Error ? projectError.message : "プロジェクトの削除に失敗しました");
    } finally {
      setDeletingProjectId("");
    }
  }

  if (authLoading) {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Box sx={{ minHeight: "100vh", ...shellBg }}>
          <Container maxWidth="sm" sx={{ minHeight: "100vh", display: "grid", placeItems: "center", py: 4 }}>
            <Card variant="outlined" sx={{ width: "100%", borderRadius: 4, boxShadow: "0 24px 48px rgba(15, 23, 42, 0.12)" }}>
              <CardContent sx={{ p: 4 }}>
                <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", textAlign: "center", gap: 2 }}>
                  <CircularProgress />
                  <Box>
                    <Typography variant="overline" color="primary">
                      D Cloud Console
                    </Typography>
                    <Typography variant="h4" sx={{ mt: 1, fontWeight: 700 }}>
                      認証状態を確認しています
                    </Typography>
                    <Typography color="text.secondary" sx={{ mt: 1 }}>
                      ログイン情報を確認して、管理画面を表示します。
                    </Typography>
                  </Box>
                </Box>
              </CardContent>
            </Card>
          </Container>
        </Box>
      </ThemeProvider>
    );
  }

  if (!currentUser) {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Box sx={{ minHeight: "100vh", ...shellBg }}>
          <Container maxWidth="sm" sx={{ minHeight: "100vh", display: "grid", placeItems: "center", py: 4 }}>
            <Card variant="outlined" sx={{ width: "100%", borderRadius: 4, overflow: "hidden", boxShadow: "0 24px 48px rgba(15, 23, 42, 0.12)" }}>
              <Box sx={{ height: 6, background: "linear-gradient(90deg, #2563eb, #7c3aed)" }} />
              <CardContent sx={{ p: 4 }}>
                <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
                  <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1.25 }}>
                      <CloudQueueOutlinedIcon sx={{ fontSize: 32, color: "primary.main" }} />
                      <Box>
                        <Typography variant="h6" sx={{ fontWeight: 700, lineHeight: 1 }}>
                          D Cloud
                        </Typography>
                        <Typography variant="body2" color="text.secondary">
                          Console
                        </Typography>
                      </Box>
                    </Box>
                    <Chip label="Infrastructure console" variant="outlined" />
                  </Box>

                  <Box>
                    <Typography variant="overline" color="primary">
                      D Cloud Console
                    </Typography>
                    <Typography variant="h4" sx={{ mt: 1, fontWeight: 700 }}>
                      プロジェクトとサービスを管理する
                    </Typography>
                    <Typography color="text.secondary" sx={{ mt: 1, lineHeight: 1.8 }}>
                      サインインすると、プロジェクトの切り替え、サービスのデプロイ、削除までこの画面から操作できます。
                    </Typography>
                  </Box>

                  <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1.5 }}>
                    <Button variant="contained" size="large" onClick={startLogin} fullWidth>
                      ログイン
                    </Button>
                    <Button variant="outlined" size="large" onClick={startRegister} fullWidth>
                      ユーザー登録
                    </Button>
                  </Box>

                  {error ? <Alert severity="error">{error}</Alert> : null}
                </Box>
              </CardContent>
            </Card>
          </Container>
        </Box>
      </ThemeProvider>
    );
  }

  const serviceStatusIcon =
    selectedStatus === "ready" ? (
      <CheckCircleIcon fontSize="small" />
    ) : selectedStatus === "loading" ? (
      <HourglassTopIcon fontSize="small" />
    ) : (
      <ErrorOutlinedIcon fontSize="small" />
    );

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Box sx={{ minHeight: "100vh", ...shellBg }}>
        <Snackbar open={Boolean(message)} autoHideDuration={3500} onClose={() => setMessage("")} anchorOrigin={{ vertical: "top", horizontal: "right" }}>
          <Alert severity="success" variant="filled" sx={{ width: "100%" }}>
            {message}
          </Alert>
        </Snackbar>

        <AppBar
          position="sticky"
          color="transparent"
          elevation={0}
          sx={{
            backdropFilter: "blur(18px)",
            backgroundColor: alpha("#ffffff", 0.78),
            borderBottom: "1px solid rgba(148, 163, 184, 0.18)"
          }}
        >
          <Toolbar sx={{ gap: 1.5, minHeight: 64 }}>
            <IconButton edge="start" onClick={() => setSidebarOpen(true)} aria-label="navigation">
              <MenuIcon />
            </IconButton>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.25, minWidth: 0 }}>
              <CloudQueueOutlinedIcon sx={{ color: "primary.main" }} />
              <Typography variant="h6" sx={{ fontWeight: 700, whiteSpace: "nowrap" }}>
                D Cloud
              </Typography>
            </Box>
            <Box sx={{ flex: 1 }} />
            <TextField
              select
              size="small"
              value={activeProjectId}
              onChange={(event) => handleProjectSelect(event.target.value)}
              disabled={projects.length === 0}
              sx={{ minWidth: { xs: 120, sm: 220 }, bgcolor: "background.paper" }}
              slotProps={{ htmlInput: { "aria-label": "プロジェクトを切り替え" } }}
            >
              {projects.map((project) => (
                <MenuItem key={project.id} value={project.id}>
                  {project.name}
                </MenuItem>
              ))}
            </TextField>
            <Button variant="outlined" startIcon={<LogoutIcon />} onClick={startLogout} sx={{ whiteSpace: "nowrap" }}>
              ログアウト
            </Button>
          </Toolbar>
        </AppBar>

        <Drawer
          open={sidebarOpen}
          onClose={() => setSidebarOpen(false)}
          variant="temporary"
          ModalProps={{ keepMounted: true }}
          slotProps={{
            paper: {
              sx: {
                width: 300,
                borderRadius: 4,
                m: 2,
                border: "1px solid rgba(148, 163, 184, 0.24)",
                boxShadow: "0 24px 48px rgba(15, 23, 42, 0.12)"
              }
            }
          }}
        >
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5, p: 2 }}>
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1.25 }}>
                <CloudQueueOutlinedIcon color="primary" />
                <Typography variant="h6" sx={{ fontWeight: 700 }}>
                  D Cloud
                </Typography>
              </Box>
              <IconButton onClick={() => setSidebarOpen(false)} aria-label="close navigation">
                <CloseIcon />
              </IconButton>
            </Box>
            <Divider />
            <List disablePadding>
              {navItems.map((item) => (
                <ListItemButton
                  key={item.id}
                  selected={route.section === item.id}
                  onClick={() => {
                    window.location.hash = `#${item.id}`;
                    if (window.matchMedia("(max-width: 760px)").matches) {
                      setSidebarOpen(false);
                    }
                  }}
                  sx={{
                    mb: 1,
                    borderRadius: 2,
                    border: "1px solid rgba(148, 163, 184, 0.24)",
                    "&.Mui-selected": {
                      bgcolor: alpha("#2563eb", 0.08),
                      borderColor: alpha("#2563eb", 0.28)
                    }
                  }}
                >
                  <ListItemIcon sx={{ minWidth: 40 }}>
                    {item.id === "home" ? <HomeOutlinedIcon /> : item.id === "container" ? <StorageOutlinedIcon /> : <CloudUploadOutlinedIcon />}
                  </ListItemIcon>
                  <ListItemText primary={item.label} />
                </ListItemButton>
              ))}
            </List>
          </Box>
        </Drawer>

        <Container maxWidth={false} sx={{ py: { xs: 2, md: 3 }, px: { xs: 1.5, sm: 2, md: 3 } }}>
          {route.section === "home" ? (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
              <Card variant="outlined" sx={{ borderRadius: 4 }}>
                <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
                  <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 2, flexWrap: "wrap" }}>
                    <Box>
                      <Typography variant="overline" color="primary">
                        プロジェクト
                      </Typography>
                      <Typography variant="h5" sx={{ fontWeight: 700, mt: 0.5 }}>
                        プロジェクト管理
                      </Typography>
                    </Box>
                    <Button variant="contained" startIcon={<AddOutlinedIcon />} onClick={() => setShowProjectCreateForm((current) => !current)}>
                      プロジェクトを作成
                    </Button>
                  </Box>

                  {showProjectCreateForm ? (
                    <Box component="form" onSubmit={handleCreateProject} sx={{ display: "grid", gap: 2, maxWidth: 560 }}>
                      <TextField
                        label="プロジェクト名"
                        value={projectName}
                        onChange={(event) => setProjectName(event.target.value)}
                        placeholder="新しいプロジェクト"
                        fullWidth
                      />
                      <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
                        <Button type="submit" variant="contained" disabled={creatingProject || !projectName.trim()}>
                          作成
                        </Button>
                      </Box>
                    </Box>
                  ) : null}

                  <Box sx={{ display: "grid", gap: 1 }}>
                    <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "minmax(180px, 1fr) minmax(0, 1fr) auto" }, gap: 1.5, px: 1.5, color: "text.secondary", fontSize: 12, fontWeight: 700 }}>
                      <Box>名前</Box>
                      <Box>ID</Box>
                      <Box />
                    </Box>

                    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.25 }}>
                      {projects.map((project) => {
                        const isActive = project.id === activeProjectId;
                        const canDelete = project.name !== "default";
                        return (
                          <Paper
                            key={project.id}
                            variant="outlined"
                            sx={{
                              p: 1.5,
                              borderRadius: 3,
                              borderColor: isActive ? alpha("#2563eb", 0.4) : "divider",
                              bgcolor: isActive ? alpha("#2563eb", 0.04) : "background.paper"
                            }}
                          >
                            <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "minmax(180px, 1fr) minmax(0, 1fr) auto" }, gap: 1.5, alignItems: "center" }}>
                              <Box sx={{ minWidth: 0 }}>
                                <Button onClick={() => handleProjectSelect(project.id)} sx={{ width: "100%", justifyContent: "flex-start", textAlign: "left", px: 0, color: "inherit" }}>
                                  <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
                                    <Typography sx={{ fontWeight: 700 }}>{project.name}</Typography>
                                    {isActive ? <Chip label="現在使用中" size="small" color="primary" variant="outlined" /> : null}
                                  </Box>
                                </Button>
                              </Box>
                              <Box sx={{ minWidth: 0 }}>
                                <Typography variant="body2" color="text.secondary" sx={{ wordBreak: "break-all" }}>
                                  {project.id}
                                </Typography>
                              </Box>
                              <Box>
                                <Button
                                  variant="outlined"
                                  color="inherit"
                                  startIcon={<DeleteOutlinedIcon />}
                                  disabled={!canDelete || deletingProjectId === project.id}
                                  onClick={() => requestDeleteProject(project.id)}
                                  fullWidth
                                >
                                  {deletingProjectId === project.id ? "削除中..." : "削除"}
                                </Button>
                              </Box>
                            </Box>
                          </Paper>
                        );
                      })}
                    </Box>
                  </Box>
                </CardContent>
              </Card>
            </Box>
          ) : route.section === "container" ? (
            <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", lg: "minmax(0, 1fr) 360px" }, gap: 3, alignItems: "start" }}>
              <Box>
                {route.selectedServiceName ? (
                  <Card variant="outlined" sx={{ borderRadius: 4 }}>
                    <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
                      {selectedService ? (
                        <>
                          <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 2, flexWrap: "wrap" }}>
                            <Box sx={{ display: "flex", alignItems: "center", gap: 1.25, minWidth: 0 }}>
                              <Box sx={{ width: 34, height: 34, borderRadius: "999px", display: "grid", placeItems: "center", bgcolor: selectedStatus === "ready" ? alpha("#16a34a", 0.12) : selectedStatus === "loading" ? alpha("#2563eb", 0.12) : alpha("#dc2626", 0.12), color: selectedStatus === "ready" ? "success.main" : selectedStatus === "loading" ? "primary.main" : "error.main" }}>
                                {serviceStatusIcon}
                              </Box>
                              <Box sx={{ minWidth: 0 }}>
                                <Typography variant="overline" color="primary">
                                  サービス詳細
                                </Typography>
                                <Typography variant="h5" sx={{ fontWeight: 700, wordBreak: "break-word" }}>
                                  {selectedService.name}
                                </Typography>
                              </Box>
                            </Box>
                            <Button startIcon={<ArrowBackIcon />} onClick={() => (window.location.hash = "#container")}>
                              一覧に戻る
                            </Button>
                          </Box>

                          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" }, gap: 1.5 }}>
                            <Paper variant="outlined" sx={{ p: 2, borderRadius: 3, bgcolor: "grey.50" }}>
                              <Typography variant="caption" color="text.secondary">
                                状態
                              </Typography>
                              <Typography sx={{ mt: 0.5, fontWeight: 600 }}>{formatServiceStatus(selectedService)}</Typography>
                            </Paper>
                            <Paper variant="outlined" sx={{ p: 2, borderRadius: 3, bgcolor: "grey.50" }}>
                              <Typography variant="caption" color="text.secondary">
                                イメージ
                              </Typography>
                              <Typography sx={{ mt: 0.5, fontWeight: 600, wordBreak: "break-all" }}>{selectedService.image}</Typography>
                            </Paper>
                            <Paper variant="outlined" sx={{ p: 2, borderRadius: 3, bgcolor: "grey.50" }}>
                              <Typography variant="caption" color="text.secondary">
                                URL
                              </Typography>
                              <Typography sx={{ mt: 0.5, fontWeight: 600, wordBreak: "break-all" }}>
                                {selectedService.url ? (
                                  <a href={selectedService.url} target="_blank" rel="noreferrer">
                                    {selectedService.url}
                                  </a>
                                ) : (
                                  "-"
                                )}
                              </Typography>
                            </Paper>
                            <Paper variant="outlined" sx={{ p: 2, borderRadius: 3, bgcolor: "grey.50" }}>
                              <Typography variant="caption" color="text.secondary">
                                作成時刻
                              </Typography>
                              <Typography sx={{ mt: 0.5, fontWeight: 600 }}>{selectedService.createdAt ?? "-"}</Typography>
                            </Paper>
                          </Box>

                          <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
                            <Button variant="contained" color="error" startIcon={<DeleteOutlinedIcon />} onClick={() => requestDelete(selectedService.name)}>
                              削除
                            </Button>
                          </Box>
                        </>
                      ) : (
                        <>
                          <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 2 }}>
                            <Typography variant="h6" sx={{ fontWeight: 700 }}>
                              サービスが見つかりません
                            </Typography>
                            <Button startIcon={<ArrowBackIcon />} onClick={() => (window.location.hash = "#container")}>
                              一覧に戻る
                            </Button>
                          </Box>
                          <Typography color="text.secondary">削除されたか、まだ同期されていません。</Typography>
                        </>
                      )}
                    </CardContent>
                  </Card>
                ) : (
                  <Card variant="outlined" sx={{ borderRadius: 4 }}>
                    <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
                      <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
                        <Box>
                          <Typography variant="overline" color="primary">
                            サービス
                          </Typography>
                          <Typography variant="h5" sx={{ fontWeight: 700, mt: 0.5 }}>
                            デプロイ済みサービス
                          </Typography>
                        </Box>
                      </Box>

                      <Box sx={{ display: "grid", gap: 0 }}>
                        <Box sx={{ display: "grid", gridTemplateColumns: "42px minmax(0, 1fr)", alignItems: "center", minHeight: 36, px: 1, color: "text.secondary", fontSize: 11, fontWeight: 700, borderBottom: "1px solid rgba(148, 163, 184, 0.18)" }}>
                          <Box />
                          <Box sx={{ display: "grid", gridTemplateColumns: "minmax(120px, max-content) 150px", columnGap: 3 }}>
                            <Box>名前</Box>
                            <Box>更新日時</Box>
                          </Box>
                        </Box>

                        <Box sx={{ borderTop: "1px solid rgba(148, 163, 184, 0.18)" }}>
                          {services.length > 0 ? (
                            services.map((service) => {
                              const status = getServiceStatus(service);
                              const statusIcon = status === "ready" ? <CheckCircleIcon fontSize="small" /> : status === "loading" ? <HourglassTopIcon fontSize="small" /> : <ErrorOutlinedIcon fontSize="small" />;
                              return (
                                <Paper
                                  key={service.name}
                                  variant="outlined"
                                  sx={{
                                    display: "grid",
                                    gridTemplateColumns: "42px minmax(0, 1fr)",
                                    alignItems: "center",
                                    minHeight: 44,
                                    px: 1,
                                    borderRadius: 0,
                                    borderLeft: 0,
                                    borderRight: 0,
                                    borderTop: 0
                                  }}
                                >
                                  <Box sx={{ display: "grid", placeItems: "center" }}>
                                    <Box sx={{ width: 22, height: 22, display: "grid", placeItems: "center", borderRadius: "999px", bgcolor: status === "ready" ? alpha("#16a34a", 0.12) : status === "loading" ? alpha("#2563eb", 0.12) : alpha("#dc2626", 0.12), color: status === "ready" ? "success.main" : status === "loading" ? "primary.main" : "error.main" }}>
                                      {statusIcon}
                                    </Box>
                                  </Box>
                                  <Box sx={{ display: "grid", gridTemplateColumns: "minmax(120px, max-content) 150px", columnGap: 3, alignItems: "center", minWidth: 0 }}>
                                    <Button component="a" href={`#container/${encodeURIComponent(service.name)}`} sx={{ justifyContent: "flex-start", textAlign: "left", color: "inherit", px: 0, minWidth: 0 }}>
                                      <Typography sx={{ fontWeight: 700, wordBreak: "break-all" }}>{service.name}</Typography>
                                    </Button>
                                    <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: "nowrap" }}>
                                      {service.updatedAt || service.createdAt ? formatServiceTimestamp(service.updatedAt || service.createdAt || "") : "-"}
                                    </Typography>
                                  </Box>
                                </Paper>
                              );
                            })
                          ) : (
                            <Paper variant="outlined" sx={{ p: 2, borderRadius: 3, borderStyle: "dashed", bgcolor: alpha("#ffffff", 0.7) }}>
                              <Typography color="text.secondary">{loading ? "読み込み中..." : "まだサービスはありません。"}</Typography>
                            </Paper>
                          )}
                        </Box>
                      </Box>
                    </CardContent>
                  </Card>
                )}
              </Box>

              <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
                <Card variant="outlined" sx={{ borderRadius: 4 }}>
                  <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
                    <Box>
                      <Typography variant="overline" color="primary">
                        作成
                      </Typography>
                      <Typography variant="h6" sx={{ fontWeight: 700, mt: 0.5 }}>
                        サービスのデプロイ
                      </Typography>
                    </Box>
                    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
                      <Button component="a" href="#deploy" variant="contained" startIcon={<CloudUploadOutlinedIcon />} fullWidth>
                        コンテナのデプロイ
                      </Button>
                      <Button component="a" href="#container" variant="outlined" startIcon={<GitHubIcon />} fullWidth>
                        リポジトリの接続
                      </Button>
                    </Box>
                  </CardContent>
                </Card>
              </Box>
            </Box>
          ) : route.section === "deploy" ? (
            <Card variant="outlined" sx={{ borderRadius: 4, maxWidth: 980 }}>
              <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
                <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
                  <Box>
                    <Typography variant="overline" color="primary">
                      サービスの作成
                    </Typography>
                    <Typography variant="h5" sx={{ fontWeight: 700, mt: 0.5 }}>
                      コンテナを作成
                    </Typography>
                  </Box>
                  <Button startIcon={<ArrowBackIcon />} onClick={() => (window.location.hash = "#home")}>
                    一覧に戻る
                  </Button>
                </Box>

                <Box component="form" onSubmit={handleSubmit} sx={{ display: "grid", gap: 2 }}>
                  <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", md: "1fr 1fr 1fr" } }}>
                    <Box sx={{ gridColumn: { xs: "auto", md: "span 2" } }}>
                      <TextField
                        label="サービス名"
                        value={form.name}
                        onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))}
                        placeholder="service-name"
                        fullWidth
                      />
                    </Box>
                    <Box>
                      <TextField
                        label="コンテナイメージのURL"
                        value={form.image}
                        onChange={(event) => setForm((current) => ({ ...current, image: event.target.value }))}
                        placeholder="ghcr.io/org/app:tag"
                        fullWidth
                      />
                    </Box>
                    <TextField
                      label="Port"
                      type="number"
                      slotProps={{ htmlInput: { min: 1, max: 65535 } }}
                      value={form.port}
                      onChange={(event) => setForm((current) => ({ ...current, port: event.target.value }))}
                      placeholder="8080"
                      fullWidth
                    />
                    <TextField
                      label="最小スケール数"
                      type="number"
                      slotProps={{ htmlInput: { min: 0, max: 20 } }}
                      value={form.minScale}
                      onChange={(event) => setForm((current) => ({ ...current, minScale: event.target.value }))}
                      placeholder="0"
                      fullWidth
                    />
                    <TextField
                      label="最大スケール数"
                      type="number"
                      slotProps={{ htmlInput: { min: 1, max: 20 } }}
                      value={form.maxScale}
                      onChange={(event) => setForm((current) => ({ ...current, maxScale: event.target.value }))}
                      placeholder="1"
                      fullWidth
                    />
                  </Box>

                  <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
                    <Button type="submit" variant="contained" disabled={submitting}>
                      {submitting ? "作成中..." : "作成"}
                    </Button>
                  </Box>
                </Box>

                {error ? <Alert severity="error">{error}</Alert> : null}
              </CardContent>
            </Card>
          ) : null}
        </Container>

        <Dialog open={Boolean(pendingDeleteName)} onClose={cancelDelete} fullWidth maxWidth="sm">
          <DialogTitle sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <DeleteOutlinedIcon color="error" />
            削除の確認
          </DialogTitle>
          <DialogContent dividers>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>
                {pendingDeleteName}
              </Typography>
              <Typography color="text.secondary">このサービスを削除しますか？</Typography>
            </Box>
          </DialogContent>
          <DialogActions sx={{ p: 2 }}>
            <Button onClick={cancelDelete} variant="outlined">
              キャンセル
            </Button>
            <Button onClick={() => confirmDelete(pendingDeleteName)} variant="contained" color="error" disabled={deletingName === pendingDeleteName}>
              {deletingName === pendingDeleteName ? "削除中..." : "削除"}
            </Button>
          </DialogActions>
        </Dialog>

        <Dialog open={Boolean(pendingProjectDeleteId)} onClose={cancelProjectDelete} fullWidth maxWidth="sm">
          <DialogTitle sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <DeleteOutlinedIcon color="error" />
            削除の確認
          </DialogTitle>
          <DialogContent dividers>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>
                プロジェクトの削除
              </Typography>
              <Typography color="text.secondary">このプロジェクトを削除しますか？</Typography>
            </Box>
          </DialogContent>
          <DialogActions sx={{ p: 2 }}>
            <Button onClick={cancelProjectDelete} variant="outlined">
              キャンセル
            </Button>
            <Button onClick={() => confirmDeleteProject(pendingProjectDeleteId)} variant="contained" color="error" disabled={deletingProjectId === pendingProjectDeleteId}>
              {deletingProjectId === pendingProjectDeleteId ? "削除中..." : "削除"}
            </Button>
          </DialogActions>
        </Dialog>
      </Box>
    </ThemeProvider>
  );
}

function getServiceStatus(service: DeployedService) {
  if (service.ready) {
    return "ready" as const;
  }

  const reason = service.reason?.toLowerCase() ?? "";
  if (
    reason.includes("pending") ||
    reason.includes("loading") ||
    reason.includes("progress") ||
    reason.includes("reconcil") ||
    reason.includes("revisionmissing") ||
    reason.includes("unknown")
  ) {
    return "loading" as const;
  }

  return "error" as const;
}

function formatServiceStatus(service: DeployedService) {
  if (service.ready) {
    return "正常";
  }

  return formatServiceReason(service.reason);
}

function formatServiceReason(reason?: string) {
  switch (reason) {
    case "RevisionMissing":
      return "リビジョンを準備中です";
    case "RevisionFailed":
      return "リビジョンの作成に失敗しました";
    case "ContainerMissing":
      return "コンテナが見つかりません";
    case "ContainerCreating":
      return "コンテナを作成中です";
    case "ImagePullBackOff":
      return "イメージの取得に失敗しました";
    case "ErrImagePull":
      return "イメージ取得エラーです";
    default:
      return "処理中です";
  }
}

function formatServiceTimestamp(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat("ja-JP", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(date);
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
