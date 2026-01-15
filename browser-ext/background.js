importScripts("config.js");

function resolveBaseUrl() {
  if (typeof globalThis.GW_BASE_URL !== "string") {
    return "";
  }
  const trimmed = globalThis.GW_BASE_URL.trim();
  if (!trimmed) {
    return "";
  }
  try {
    const parsed = new URL(trimmed);
    return parsed.origin;
  } catch (error) {
    return "";
  }
}

const baseUrl = resolveBaseUrl();
const AUTH_COMPLETE_PREFIX = baseUrl ? `${baseUrl}/ext/complete` : "";
const TOKEN_KEY = "gwToken";

chrome.tabs.onUpdated.addListener((tabId, changeInfo) => {
  if (!changeInfo.url || !AUTH_COMPLETE_PREFIX) {
    return;
  }

  if (!changeInfo.url.startsWith(AUTH_COMPLETE_PREFIX)) {
    return;
  }

  try {
    const url = new URL(changeInfo.url);
    const token = url.searchParams.get("token");
    if (token) {
      chrome.storage.local.set({ [TOKEN_KEY]: token });
    }

    if (url.searchParams.has("token")) {
      url.searchParams.delete("token");
      url.searchParams.set("status", "ok");
      chrome.tabs.update(tabId, { url: url.toString() });
    }
  } catch (error) {
    console.error("Groundwave Connector auth parsing failed", error);
  }
});
