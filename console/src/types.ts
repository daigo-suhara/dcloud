export type PlatformResponse = {
  namespace: string;
  user: string;
  projectId: string;
  containers: DeployedService[];
};

export type ProjectsResponse = {
  user: string;
  projects: Project[];
};

export type AuthUser = {
  id: string;
  username: string;
  email?: string;
  name?: string;
};

export type AuthForm = {
  username: string;
  password: string;
  email: string;
  name: string;
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
  section: "home" | "container" | "deploy" | "project-create" | "repository";
  selectedServiceName: string | null;
};

export type RepositoryConfig = {
  projectId: string;
  userId: string;
  repositoryOwner: string;
  repositoryName: string;
  repositoryBranch: string;
  connectedAt: string;
  updatedAt: string;
};

export type RepositoryForm = {
  repositoryOwner: string;
  repositoryName: string;
  repositoryBranch: string;
};

export const initialForm: DeployForm = {
  name: "",
  image: "",
  port: "8080",
  minScale: "0",
  maxScale: "20"
};

export const initialAuthForm: AuthForm = {
  username: "",
  password: "",
  email: "",
  name: ""
};

export const navItems = [
  { id: "home", label: "ホーム" },
  { id: "container", label: "コンテナ" },
  { id: "deploy", label: "仮想マシン" }
] as const;
