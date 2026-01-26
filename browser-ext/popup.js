/*
SPDX-FileCopyrightText: 2025 Humaid Alqasimi
SPDX-License-Identifier: Apache-2.0
*/

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
const AUTH_URL = baseUrl ? `${baseUrl}/ext/auth` : "";
const VALIDATE_URL = baseUrl ? `${baseUrl}/ext/validate` : "";
const TOKEN_KEY = "gwToken";
const HEADER_NAME = "X-Groundwave-Token";

const statusDot = document.getElementById("status-dot");
const statusText = document.getElementById("status-text");
const authButton = document.getElementById("auth-button");

function setStatus(isAuthenticated) {
  statusDot.classList.toggle("status-dot--ok", isAuthenticated);
  statusDot.classList.toggle("status-dot--bad", !isAuthenticated);
  statusText.textContent = isAuthenticated ? "Authenticated" : "Not authenticated";
}

async function getToken() {
  const result = await chrome.storage.local.get(TOKEN_KEY);
  return result[TOKEN_KEY] || null;
}

async function clearToken() {
  await chrome.storage.local.remove(TOKEN_KEY);
}

async function validateToken(token) {
  if (!VALIDATE_URL) {
    return false;
  }
  try {
    const response = await fetch(VALIDATE_URL, {
      method: "GET",
      redirect: "manual",
      headers: {
        [HEADER_NAME]: token,
      },
    });
    return response.ok;
  } catch (error) {
    return false;
  }
}

async function refreshStatus() {
  const token = await getToken();
  if (!token || !VALIDATE_URL) {
    setStatus(false);
    return;
  }

  const isValid = await validateToken(token);
  if (!isValid) {
    await clearToken();
  }
  setStatus(isValid);
}

authButton.addEventListener("click", () => {
  if (!AUTH_URL) {
    return;
  }
  chrome.tabs.create({ url: AUTH_URL });
});

refreshStatus();
