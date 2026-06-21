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
  email: string;
  password: string;
};

export type ComputeForm = {
  name: string;
  image: string;
  cpu: string;
  memory: string;
};

export type Project = {
  id: string;
  name: string;
  owner: string;
  createdAt: string;
  deleting?: boolean;
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
  customDomain?: string | null;
  domainStatus?: "ready" | "pending" | "error" | null;
  domainStatusReason?: string | null;
  domainCnameTarget?: string | null;
  port?: number;
  minScale?: number;
  maxScale?: number;
  startupScript?: string | null;
  env?: { name: string; value: string }[] | null;
};

export type ComputeMachine = {
  name: string;
  image: string;
  cpu: string;
  memory: string;
  ready: boolean;
  status?: string;
  reason?: string;
  createdAt?: string;
  updatedAt?: string;
  namespace: string;
  projectId?: string;
  generation?: number;
};

export type EnvVarEntry = { name: string; value: string };

export type DeployForm = {
  name: string;
  image: string;
  port: string;
  minScale: string;
  maxScale: string;
  startupScript: string;
  env: EnvVarEntry[];
};

export type UpdateForm = {
  image: string;
  port: string;
  minScale: string;
  maxScale: string;
  startupScript: string;
  env: EnvVarEntry[];
};

export type Bucket = {
  name: string;
  endpoint: string;
  ready: boolean;
  status: string;
  createdAt?: string;
  projectId?: string;
};

export type DatabaseInstance = {
  name: string;
  type: string;
  version: string;
  cpu: string;
  memory: string;
  storage: string;
  ready: boolean;
  status: string;
  createdAt?: string;
  projectId?: string;
};

export type BucketCreateForm = {
  name: string;
};

export type DatabaseCreateForm = {
  name: string;
  type: string;
  version: string;
  cpu: string;
  memory: string;
  storage: string;
};

export type RouteState = {
  section: "home" | "container" | "compute" | "compute-create" | "deploy" | "project-create" | "repository" | "storage" | "database";
  selectedServiceName: string | null;
  selectedComputeMachineName: string | null;
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
  maxScale: "20",
  startupScript: "",
  env: []
};

export const initialAuthForm: AuthForm = {
  email: "",
  password: ""
};

export const initialComputeForm: ComputeForm = {
  name: "",
  image: "quay.io/containerdisks/fedora:latest",
  cpu: "1",
  memory: "1Gi"
};

export const navItems = [
  { id: "home", label: "ホーム" },
  { id: "container", label: "コンテナ" },
  { id: "compute", label: "仮想マシン" },
  { id: "storage", label: "ストレージ" },
  { id: "database", label: "データベース" }
] as const;
