const TOKEN_KEY = "gwToken";
const HEADER_NAME = "X-Groundwave-Token";

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
const hasBaseUrl = Boolean(baseUrl);

console.log("[Groundwave Connector] content script loaded", {
  baseUrl,
  hasBaseUrl,
  href: window.location.href,
});

const lookupCache = new Map();
const pendingLookup = new Set();
let batchTimer = null;

let contactsCache = null;
let contactsCachePromise = null;
let modalOverlay = null;
let modalList = null;
let modalInput = null;
let modalStatus = null;
let modalActiveIndicator = null;
let modalActiveUrl = null;

const CARD_SELECTOR = 'div[data-view-name="connections-list"]';
const PROFILE_LINK_SELECTOR = 'a[data-view-name="connections-profile"]';
const LAZY_COLUMN_SELECTOR = 'div[data-component-type="LazyColumn"]';
const PROFILE_CONTENT_SELECTOR = "#profile-content";
const MESSAGE_BUTTON_SELECTOR = '[data-view-name="message-button"]';
const PROFILE_NAME_SELECTOR = "main h1";
const PROFILE_DISTANCE_SELECTOR = "span.dist-value";
const MODAL_OVERLAY_CLASS = "gw-connector-modal";
const MODAL_LIST_CLASS = "gw-connector-modal__list";
const MODAL_INPUT_CLASS = "gw-connector-modal__search";
const MODAL_STATUS_CLASS = "gw-connector-modal__status";

function normalizeLinkedInUrl(rawUrl) {
  if (!rawUrl) {
    return null;
  }

  try {
    const url = new URL(rawUrl, window.location.origin);
    let host = url.hostname.toLowerCase();
    if (host.startsWith("www.")) {
      host = host.slice(4);
    }
    if (host !== "linkedin.com") {
      return null;
    }

    let path = url.pathname.replace(/\/+$/, "").toLowerCase();
    if (!path.startsWith("/in/")) {
      return null;
    }

    const parts = path.split("/").filter(Boolean);
    if (parts.length < 2) {
      return null;
    }

    return `https://linkedin.com/in/${parts[1]}`;
  } catch (error) {
    return null;
  }
}

async function getToken() {
  const result = await chrome.storage.local.get(TOKEN_KEY);
  return result[TOKEN_KEY] || null;
}

async function clearToken() {
  await chrome.storage.local.remove(TOKEN_KEY);
}

function ensureBatchTimer() {
  if (batchTimer) {
    return;
  }
  batchTimer = window.setTimeout(processBatch, 250);
}

function scheduleLookup(normalizedUrl) {
  if (lookupCache.has(normalizedUrl) || !hasBaseUrl) {
    return;
  }
  pendingLookup.add(normalizedUrl);
  ensureBatchTimer();
}

async function processBatch() {
  batchTimer = null;
  if (pendingLookup.size === 0) {
    return;
  }

  const token = await getToken();
  if (!token || !hasBaseUrl) {
    console.log("[Groundwave Connector] missing token or base URL", { token: Boolean(token), hasBaseUrl });
    return;
  }

  const urls = Array.from(pendingLookup);
  pendingLookup.clear();
  console.log("[Groundwave Connector] lookup batch", { count: urls.length, baseUrl });

  try {
    const response = await fetch(`${baseUrl}/ext/linkedin-lookup`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        [HEADER_NAME]: token,
      },
      body: JSON.stringify({ urls }),
    });

    if (!response.ok) {
      if (response.status === 401) {
        await clearToken();
      }
      return;
    }

    const data = await response.json();
    const matches = data.matches || {};
    const contacts = data.contacts || {};
    urls.forEach((url) => {
      const exists = Boolean(matches[url]);
      lookupCache.set(url, { exists, contactId: contacts[url] || null });
    });
    updateAllIndicators();
  } catch (error) {
    console.warn("Groundwave lookup failed", error);
  }
}

function getLookupEntry(normalizedUrl) {
  if (!lookupCache.has(normalizedUrl)) {
    return null;
  }
  const entry = lookupCache.get(normalizedUrl);
  return typeof entry === "boolean" ? { exists: entry, contactId: null } : entry;
}

function isProfilePage() {
  return window.location.pathname.toLowerCase().startsWith("/in/");
}

async function fetchContactsWithoutLinkedIn() {
  if (contactsCache) {
    return contactsCache;
  }

  if (contactsCachePromise) {
    return contactsCachePromise;
  }

  contactsCachePromise = (async () => {
    const token = await getToken();
    if (!token || !hasBaseUrl) {
      return [];
    }

    const response = await fetch(`${baseUrl}/ext/contacts-no-linkedin`, {
      method: "GET",
      headers: {
        [HEADER_NAME]: token,
      },
    });

    if (!response.ok) {
      return [];
    }

    const data = await response.json();
    contactsCache = Array.isArray(data.contacts) ? data.contacts : [];
    return contactsCache;
  })();

  return contactsCachePromise;
}

function closeAssignModal() {
  if (!modalOverlay) {
    return;
  }
  modalOverlay.style.display = "none";
  modalActiveIndicator = null;
  modalActiveUrl = null;
  if (modalInput) {
    modalInput.value = "";
  }
}

function ensureAssignModal() {
  if (modalOverlay) {
    return;
  }

  modalOverlay = document.createElement("div");
  modalOverlay.className = MODAL_OVERLAY_CLASS;
  Object.assign(modalOverlay.style, {
    position: "fixed",
    inset: "0",
    background: "rgba(0, 0, 0, 0.4)",
    display: "none",
    alignItems: "center",
    justifyContent: "center",
    zIndex: "99999",
  });

  const panel = document.createElement("div");
  Object.assign(panel.style, {
    background: "#fff",
    padding: "16px",
    borderRadius: "8px",
    width: "320px",
    maxHeight: "70vh",
    display: "flex",
    flexDirection: "column",
    gap: "8px",
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
  });

  const title = document.createElement("div");
  title.textContent = "Link to Groundwave contact";
  title.style.fontWeight = "600";

  modalInput = document.createElement("input");
  modalInput.className = MODAL_INPUT_CLASS;
  modalInput.type = "text";
  modalInput.placeholder = "Search contacts...";
  modalInput.style.padding = "6px 8px";
  modalInput.style.border = "1px solid #ccc";
  modalInput.style.borderRadius = "6px";

  modalStatus = document.createElement("div");
  modalStatus.className = MODAL_STATUS_CLASS;
  modalStatus.style.fontSize = "12px";
  modalStatus.style.color = "#666";

  modalList = document.createElement("div");
  modalList.className = MODAL_LIST_CLASS;
  Object.assign(modalList.style, {
    overflowY: "auto",
    border: "1px solid #eee",
    borderRadius: "6px",
    padding: "4px",
    maxHeight: "40vh",
  });

  const closeButton = document.createElement("button");
  closeButton.type = "button";
  closeButton.textContent = "Close";
  Object.assign(closeButton.style, {
    alignSelf: "flex-end",
    padding: "6px 10px",
    borderRadius: "6px",
    border: "1px solid #ccc",
    background: "#f7f7f7",
    cursor: "pointer",
  });

  closeButton.addEventListener("click", (event) => {
    event.preventDefault();
    closeAssignModal();
  });

  modalOverlay.addEventListener("click", (event) => {
    if (event.target === modalOverlay) {
      closeAssignModal();
    }
  });

  modalInput.addEventListener("input", () => {
    renderAssignList();
  });

  panel.append(title, modalInput, modalStatus, modalList, closeButton);
  modalOverlay.append(panel);
  document.body.append(modalOverlay);
}

function renderAssignList() {
  if (!modalList || !modalStatus) {
    return;
  }

  const query = modalInput ? modalInput.value.trim().toLowerCase() : "";
  const contacts = contactsCache || [];
  const filtered = query
    ? contacts.filter((contact) => contact.name.toLowerCase().includes(query))
    : contacts;

  modalList.innerHTML = "";

  if (filtered.length === 0) {
    modalStatus.textContent = "No contacts found.";
    return;
  }

  modalStatus.textContent = `${filtered.length} contact${filtered.length === 1 ? "" : "s"}`;

  filtered.forEach((contact) => {
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = contact.name;
    Object.assign(button.style, {
      display: "block",
      width: "100%",
      textAlign: "left",
      padding: "6px 8px",
      border: "none",
      background: "transparent",
      cursor: "pointer",
    });
    button.addEventListener("click", (event) => {
      event.preventDefault();
      selectContactForAssign(contact);
    });
    button.addEventListener("mouseenter", () => {
      button.style.background = "#f3f2ef";
    });
    button.addEventListener("mouseleave", () => {
      button.style.background = "transparent";
    });
    modalList.append(button);
  });
}

async function selectContactForAssign(contact) {
  if (!modalActiveUrl || !modalActiveIndicator) {
    return;
  }

  const confirmed = window.confirm(`Link this LinkedIn profile to ${contact.name}?`);
  if (!confirmed) {
    return;
  }

  const token = await getToken();
  if (!token || !hasBaseUrl) {
    window.alert("Groundwave connector is not authenticated.");
    return;
  }

  modalStatus.textContent = "Linking...";

  const response = await fetch(`${baseUrl}/ext/linkedin-assign`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      [HEADER_NAME]: token,
    },
    body: JSON.stringify({ contactId: contact.id, url: modalActiveUrl }),
  });

  if (!response.ok) {
    modalStatus.textContent = "Failed to link contact.";
    return;
  }

  const entry = { exists: true, contactId: contact.id };
  lookupCache.set(modalActiveUrl, entry);
  updateIndicatorAfterAssign(modalActiveIndicator, entry, modalActiveUrl);
  contactsCache = (contactsCache || []).filter((item) => item.id !== contact.id);
  closeAssignModal();
}

async function openAssignModal(normalizedUrl, indicator) {
  ensureAssignModal();
  modalActiveIndicator = indicator;
  modalActiveUrl = normalizedUrl;
  modalOverlay.style.display = "flex";
  modalStatus.textContent = "Loading contacts...";

  const contacts = await fetchContactsWithoutLinkedIn();
  contactsCache = contacts;
  modalStatus.textContent = contacts.length === 0 ? "No available contacts." : "";
  renderAssignList();
  if (modalInput) {
    modalInput.focus();
  }
}

function updateIndicatorAfterAssign(indicator, entry, normalizedUrl) {
  const newIndicator = buildIndicator(entry, normalizedUrl);
  indicator.replaceWith(newIndicator);
}

function buildIndicator(entry, normalizedUrl) {
  const contactUrl =
    entry.exists && entry.contactId && hasBaseUrl
      ? `${baseUrl}/contact/${entry.contactId}`
      : null;
  const indicator = document.createElement(contactUrl ? "a" : "span");
  indicator.className = "gw-connector-indicator";
  indicator.classList.add(entry.exists ? "gw-connector--found" : "gw-connector--missing");
  indicator.textContent = entry.exists ? "✓" : "✕";
  indicator.title = entry.exists
    ? contactUrl
      ? "Open in Groundwave"
      : "Found in Groundwave"
    : "Not in Groundwave";

  if (normalizedUrl) {
    indicator.dataset.gwUrl = normalizedUrl;
  }

  if (contactUrl) {
    indicator.href = contactUrl;
    indicator.classList.add("gw-connector--clickable");
  }

  if (!entry.exists && normalizedUrl) {
    indicator.addEventListener("click", (event) => {
      event.preventDefault();
      event.stopPropagation();
      openAssignModal(normalizedUrl, indicator);
    });
  } else {
    indicator.addEventListener("click", (event) => {
      event.stopPropagation();
    });
  }

  return indicator;
}

function insertIndicator(card, entry, normalizedUrl) {
  if (card.querySelector(".gw-connector-indicator")) {
    return;
  }

  const messageButton = card.querySelector(MESSAGE_BUTTON_SELECTOR);
  const container = messageButton?.closest("div");
  if (!container) {
    return;
  }

  const indicator = buildIndicator(entry, normalizedUrl);
  const actionButton = container.querySelector("a, button");
  if (actionButton) {
    container.insertBefore(indicator, actionButton);
  } else {
    container.prepend(indicator);
  }
}

function updateCard(card) {
  if (!card.dataset.gwUrl) {
    return;
  }

  const normalizedUrl = card.dataset.gwUrl;
  const entry = getLookupEntry(normalizedUrl);
  if (!entry) {
    return;
  }

  insertIndicator(card, entry, normalizedUrl);
}

function updateAllIndicators() {
  document.querySelectorAll(CARD_SELECTOR).forEach((card) => {
    updateCard(card);
  });
  updateProfileIndicator();
}

function processCard(card) {
  if (card.dataset.gwProcessed) {
    return;
  }

  const profileLink = card.querySelector(PROFILE_LINK_SELECTOR);
  if (!profileLink) {
    card.dataset.gwProcessed = "true";
    console.log("[Groundwave Connector] no profile link", card);
    return;
  }

  const normalizedUrl = normalizeLinkedInUrl(profileLink.href);
  if (!normalizedUrl) {
    card.dataset.gwProcessed = "true";
    console.log("[Groundwave Connector] skipped non-LinkedIn URL", profileLink.href);
    return;
  }

  card.dataset.gwProcessed = "true";
  card.dataset.gwUrl = normalizedUrl;
  console.log("[Groundwave Connector] queued", normalizedUrl);

  const entry = getLookupEntry(normalizedUrl);
  if (entry) {
    insertIndicator(card, entry, normalizedUrl);
  } else {
    scheduleLookup(normalizedUrl);
  }
}

function scanExistingCards() {
  const root = document.querySelector(LAZY_COLUMN_SELECTOR);
  if (!root) {
    return;
  }
  const cards = root.querySelectorAll(CARD_SELECTOR);
  console.log("[Groundwave Connector] scanning cards", { count: cards.length });
  cards.forEach((card) => {
    if (!card.dataset.gwProcessed) {
      processCard(card);
      return;
    }
    if (card.dataset.gwUrl && !lookupCache.has(card.dataset.gwUrl)) {
      scheduleLookup(card.dataset.gwUrl);
    }
  });
}

function updateProfileIndicator() {
  if (!isProfilePage()) {
    return;
  }

  const root = document.querySelector(PROFILE_CONTENT_SELECTOR) || document;
  const nameElement = root.querySelector(PROFILE_NAME_SELECTOR);
  if (!nameElement) {
    return;
  }

  const distanceElement = root.querySelector(PROFILE_DISTANCE_SELECTOR);
  const anchorElement = nameElement.closest("a") || nameElement;
  const indicatorTarget = distanceElement || anchorElement;
  if (indicatorTarget.dataset.gwIndicator) {
    return;
  }

  const normalizedUrl = normalizeLinkedInUrl(window.location.href);
  if (!normalizedUrl) {
    return;
  }

  const entry = getLookupEntry(normalizedUrl);
  if (!entry) {
    scheduleLookup(normalizedUrl);
    return;
  }

  const indicator = buildIndicator(entry, normalizedUrl);
  indicatorTarget.insertAdjacentElement("afterend", indicator);
  indicatorTarget.dataset.gwIndicator = "true";
}

const observer = new MutationObserver((mutations) => {
  mutations.forEach((mutation) => {
    mutation.addedNodes.forEach((node) => {
      if (!(node instanceof HTMLElement)) {
        return;
      }

      if (node.matches?.(CARD_SELECTOR)) {
        processCard(node);
      } else {
        node.querySelectorAll?.(CARD_SELECTOR).forEach((card) => {
          processCard(card);
        });
      }
    });
  });
});

const observerTarget =
  document.querySelector(LAZY_COLUMN_SELECTOR) ||
  document.querySelector(PROFILE_CONTENT_SELECTOR) ||
  document.body;
observer.observe(observerTarget, { childList: true, subtree: true });
scanExistingCards();
updateProfileIndicator();

const periodicScan = window.setInterval(() => {
  scanExistingCards();
  updateProfileIndicator();
}, 1500);

chrome.storage.onChanged.addListener((changes, area) => {
  if (area !== "local" || !changes[TOKEN_KEY]) {
    return;
  }
  scanExistingCards();
  updateProfileIndicator();
  ensureBatchTimer();
});

window.addEventListener("beforeunload", () => {
  window.clearInterval(periodicScan);
});
