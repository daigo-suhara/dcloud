import type { DeployedService, RouteState } from "./types";
import type { ComputeMachine } from "./types";

export function parseRoute(pathname: string): RouteState {
  const route = pathname.replace(/^\/+/, "");

  if (!route) {
    return { section: "home", selectedServiceName: null, selectedComputeMachineName: null, selectedDatabaseName: null };
  }

  const [section, ...rest] = route.split("/");
  const normalizedSection = section;

  if (normalizedSection === "container" && rest[0] === "deploy") {
    return { section: "deploy", selectedServiceName: null, selectedComputeMachineName: null, selectedDatabaseName: null };
  }
  if (normalizedSection === "compute") {
    if (rest[0] === "new") {
      return { section: "compute-create", selectedServiceName: null, selectedComputeMachineName: null, selectedDatabaseName: null };
    }
    if (rest.length > 0) {
      return {
        section: "compute",
        selectedServiceName: null,
        selectedComputeMachineName: decodeURIComponent(rest.join("/")),
        selectedDatabaseName: null
      };
    }
    return { section: "compute", selectedServiceName: null, selectedComputeMachineName: null, selectedDatabaseName: null };
  }
  if (normalizedSection === "container" && rest[0] === "repository") {
    return { section: "repository", selectedServiceName: null, selectedComputeMachineName: null, selectedDatabaseName: null };
  }
  if (normalizedSection === "container" && rest.length > 0) {
    return {
      section: "container",
      selectedServiceName: decodeURIComponent(rest.join("/")),
      selectedComputeMachineName: null,
      selectedDatabaseName: null
    };
  }
  if (normalizedSection === "database" && rest.length > 0) {
    return {
      section: "database-detail",
      selectedServiceName: null,
      selectedComputeMachineName: null,
      selectedDatabaseName: decodeURIComponent(rest.join("/"))
    };
  }

  if (normalizedSection === "home" || normalizedSection === "container" || normalizedSection === "deploy" || normalizedSection === "compute" || normalizedSection === "compute-create" || normalizedSection === "project-create" || normalizedSection === "repository" || normalizedSection === "storage" || normalizedSection === "database") {
    return { section: normalizedSection, selectedServiceName: null, selectedComputeMachineName: null, selectedDatabaseName: null };
  }

  return { section: "home", selectedServiceName: null, selectedComputeMachineName: null, selectedDatabaseName: null };
}

export function getServiceStatus(service: DeployedService) {
  if (service.ready) {
    return "ready" as const;
  }

  const reason = service.reason?.toLowerCase() ?? "";
  if (!reason) {
    return "loading" as const;
  }
  if (
    reason.includes("pending") ||
    reason.includes("loading") ||
    reason.includes("progress") ||
    reason.includes("creating") ||
    reason.includes("reconcil") ||
    reason.includes("revisionmissing") ||
    reason.includes("unknown")
  ) {
    return "loading" as const;
  }

  return "error" as const;
}

export function formatServiceStatus(service: DeployedService) {
  if (service.ready) {
    return "正常";
  }

  return formatServiceReason(service.reason);
}

export function formatServiceReason(reason?: string) {
  switch (reason) {
    case "RevisionMissing":
      return "リビジョンを準備中";
    case "RevisionFailed":
      return "リビジョンの作成に失敗";
    case "ContainerMissing":
      return "コンテナを準備中";
    case "ContainerCreating":
      return "コンテナを作成中";
    case "ImagePullBackOff":
      return "イメージ取得に失敗";
    case "ErrImagePull":
      return "イメージ取得エラー";
    default:
      return "処理中";
  }
}

export function formatServiceTimestamp(value: string) {
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

export function formatComputeStatus(machine: ComputeMachine) {
  if (machine.ready) {
    return "正常";
  }

  const status = machine.status?.trim();
  if (status) {
    switch (status) {
      case "Provisioning":
        return "準備中";
      case "Starting":
        return "起動中";
      case "Stopping":
        return "停止中";
      case "Stopped":
        return "停止中";
      case "Error":
        return "エラー";
      default:
        return status;
    }
  }

  return machine.reason?.trim() || "処理中";
}

export function formatComputeTimestamp(value?: string) {
  if (!value) {
    return "-";
  }

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
