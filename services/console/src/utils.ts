import type { DeployedService, RouteState } from "./types";

export function parseRoute(hash: string): RouteState {
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

  if (normalizedSection === "home" || normalizedSection === "container" || normalizedSection === "deploy") {
    return { section: normalizedSection, selectedServiceName: null };
  }

  return { section: "home", selectedServiceName: null };
}

export function getServiceStatus(service: DeployedService) {
  if (service.ready) {
    return "ready" as const;
  }

  const reason = service.reason?.toLowerCase() ?? "";
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
