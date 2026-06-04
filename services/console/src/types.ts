export type PlatformResponse = {
  namespace: string;
  user: string;
  projectId: string;
  services: DeployedService[];
};

export type ProjectsResponse = {
  user: string;
  projects: Project[];
  defaultProjectId: string;
};

export type AuthUser = {
  id: string;
  username: string;
  email?: string;
  name?: string;
};

export type Project = {
  id: string;
  name: string;
  owner: string;
  createdAt: string;
};

export type DeployedService = {
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

export type DeployForm = {
  name: string;
  image: string;
  port: string;
  minScale: string;
  maxScale: string;
};

export type RouteState = {
  section: "home" | "container" | "deploy";
  selectedServiceName: string | null;
};

export const initialForm: DeployForm = {
  name: "",
  image: "",
  port: "8080",
  minScale: "0",
  maxScale: "20"
};

export const navItems = [
  { id: "home", label: "ホーム" },
  { id: "container", label: "コンテナ" },
  { id: "deploy", label: "仮想マシン" }
] as const;
