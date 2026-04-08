            function getText(key) {
                return i18nResources[currentLanguage][key] || key;
            }

            function escapeHTML(value) {
                return String(value ?? "").replace(
                    /[&<>"']/g,
                    (char) =>
                        ({
                            "&": "&amp;",
                            "<": "&lt;",
                            ">": "&gt;",
                            '"': "&quot;",
                            "'": "&#39;",
                        })[char],
                );
            }

            function encodePayload(value) {
                return encodeURIComponent(JSON.stringify(value));
            }

            function decodePayload(payload) {
                try {
                    return JSON.parse(decodeURIComponent(payload));
                } catch (error) {
                    console.error("Decode payload failed:", error);
                    return null;
                }
            }

            function formatPlainValue(value, fallbackKey = "not_set") {
                const text = String(value ?? "").trim();
                return text === ""
                    ? `<span class="muted-text">${escapeHTML(getText(fallbackKey))}</span>`
                    : escapeHTML(text);
            }

            function formatDateTimeValue(value, fallbackKey = "not_set") {
                const text = String(value ?? "").trim();
                if (text === "") {
                    return `<span class="muted-text">${escapeHTML(
                        getText(fallbackKey),
                    )}</span>`;
                }
                const date = new Date(text);
                if (Number.isNaN(date.getTime())) {
                    return escapeHTML(text);
                }
                const pad = (num) => String(num).padStart(2, "0");
                return escapeHTML(
                    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
                        date.getDate(),
                    )} ${pad(date.getHours())}:${pad(
                        date.getMinutes(),
                    )}:${pad(date.getSeconds())}`,
                );
            }


            function getDisplayText(value, fallbackKey = "not_set") {
                const text = String(value ?? "").trim();
                return text === "" ? getText(fallbackKey) : text;
            }


            function renderPill(text, className = "meta-pill") {
                return `<span class="${className}">${text}</span>`;
            }

            function renderStatusBadge(isActive, activeKey, inactiveKey) {
                return `<span class="status-badge ${isActive ? "status-badge-enabled" : "status-badge-disabled"}">${
                    isActive
                        ? renderBadgeIcon("enabled")
                        : renderBadgeIcon("disabled")
                }<span>${escapeHTML(getText(isActive ? activeKey : inactiveKey))}</span></span>`;
            }

            function renderOnlineBadge(isOnline) {
                return `<span class="status-badge ${isOnline ? "status-badge-online" : "status-badge-offline"}">${
                    isOnline
                        ? renderBadgeIcon("online")
                        : renderBadgeIcon("offline")
                }<span>${escapeHTML(getText(isOnline ? "online" : "offline"))}</span></span>`;
            }

            function renderCompressionBadge(isCompressed) {
                return `<span class="compression-pill ${isCompressed ? "compression-pill-on" : "compression-pill-off"}">${
                    isCompressed
                        ? renderBadgeIcon("compress_on")
                        : renderBadgeIcon("compress_off")
                }<span>${escapeHTML(getText(isCompressed ? "compressed" : "uncompressed"))}</span></span>`;
            }

            function renderProtocolChip(type) {
                const key = ["tcp", "udp", "socks5", "http", "shadowsocks"][
                    type
                ];
                return `<span class="protocol-chip">${escapeHTML(
                    key ? getText(key) : getText("not_set"),
                )}</span>`;
            }

            function renderBadgeIcon(name) {
                switch (name) {
                    case "enabled":
                        return `<span class="badge-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"></path></svg></span>`;
                    case "disabled":
                        return `<span class="badge-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"></circle><path d="M9 9l6 6M15 9l-6 6"></path></svg></span>`;
                    case "online":
                        return `<span class="badge-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 18h.01"></path><path d="M8.5 14.5a5 5 0 0 1 7 0"></path><path d="M5 11a10 10 0 0 1 14 0"></path></svg></span>`;
                    case "offline":
                        return `<span class="badge-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 3l18 18"></path><path d="M8.5 14.5a5 5 0 0 1 1.85-.99"></path><path d="M5 11a10 10 0 0 1 6.19-2.86"></path><path d="M15.61 12.61A5 5 0 0 1 16.5 14.5"></path><path d="M18.85 11.85A10 10 0 0 0 15 9.63"></path></svg></span>`;
                    case "compress_on":
                        return `<span class="badge-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 9l-3 3-3-3"></path><path d="M9 15l3-3 3 3"></path><path d="M5 5h14v14H5z"></path></svg></span>`;
                    case "compress_off":
                        return `<span class="badge-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 9l3-3 3 3"></path><path d="M15 15l-3 3-3-3"></path><path d="M5 5h14v14H5z"></path></svg></span>`;
                    default:
                        return "";
                }
            }

            function renderActionIcon(name) {
                switch (name) {
                    case "edit":
                        return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 20h9"></path><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5z"></path></svg>`;
                    case "delete":
                        return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 6h18"></path><path d="M8 6V4h8v2"></path><path d="M19 6l-1 14H6L5 6"></path><path d="M10 11v6M14 11v6"></path></svg>`;
                    case "enable":
                        return `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M8 5v14l11-7z"></path></svg>`;
                    case "disable":
                        return `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M7 5h4v14H7zM13 5h4v14h-4z"></path></svg>`;
                    case "download":
                        return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3v12"></path><path d="m7 10 5 5 5-5"></path><path d="M5 21h14"></path></svg>`;
                    default:
                        return "";
                }
            }

            function renderActionButton(
                className,
                title,
                onclick,
                iconName,
                disabled = false,
            ) {
                return `<button class="action-button ${className}" type="button" title="${escapeHTML(
                    title,
                )}" aria-label="${escapeHTML(title)}" ${
                    disabled
                        ? 'disabled aria-disabled="true"'
                        : `onclick="${onclick}"`
                }>${renderActionIcon(iconName)}</button>`;
            }

            function renderPasswordEyeIcon(visible) {
                if (visible) {
                    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 3l18 18"></path><path d="M10.58 10.58A3 3 0 0 0 9 12a3 3 0 0 0 4.42 2.63"></path><path d="M9.88 5.09A10.94 10.94 0 0 1 12 5c6.5 0 10 7 10 7a18.73 18.73 0 0 1-3.04 3.81"></path><path d="M6.61 6.61A18.74 18.74 0 0 0 2 12s3.5 7 10 7a10.94 10.94 0 0 0 5.39-1.39"></path></svg>`;
                }
                return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6-10-6-10-6Z"></path><circle cx="12" cy="12" r="3"></circle></svg>`;
            }

            function updateGenerateKeyButtons() {
                const label = getText("generate_key_button");
                document
                    .querySelectorAll("[data-generate-key-button]")
                    .forEach((button) => {
                        button.setAttribute("aria-label", label);
                        button.setAttribute("title", label);
                    });
            }


            function renderEmptyState(colSpan, messageKey) {
                return `<tr class="empty-state-row"><td colspan="${colSpan}">${escapeHTML(
                    getText(messageKey),
                )}</td></tr>`;
            }

            function getSortIndicator(sortState, key) {
                if (sortState.key !== key) {
                    return "↕";
                }
                return sortState.direction === "asc" ? "↑" : "↓";
            }

            function renderSortableHeader(labelKey, sortState, key, handler) {
                const activeClass =
                    sortState.key === key
                        ? "sortable-header active"
                        : "sortable-header";
                return `<th class="${activeClass}" onclick="${handler}('${key}')"><span class="th-content"><span>${escapeHTML(
                    getText(labelKey),
                )}</span><span class="sort-indicator">${getSortIndicator(
                    sortState,
                    key,
                )}</span></span></th>`;
            }

            function normalizeSortValue(value) {
                if (typeof value === "boolean") {
                    return value ? 1 : 0;
                }
                if (typeof value === "number") {
                    return value;
                }
                return String(value ?? "")
                    .trim()
                    .toLowerCase();
            }

            function sortCollection(items, sortState, valueGetter) {
                return [...items].sort((left, right) => {
                    const a = normalizeSortValue(
                        valueGetter(left, sortState.key),
                    );
                    const b = normalizeSortValue(
                        valueGetter(right, sortState.key),
                    );
                    let result = 0;
                    if (typeof a === "number" && typeof b === "number") {
                        result = a - b;
                    } else {
                        result = String(a).localeCompare(String(b), "zh-CN", {
                            numeric: true,
                            sensitivity: "base",
                        });
                    }
                    if (result === 0) {
                        const fallbackA = normalizeSortValue(
                            valueGetter(left, "id"),
                        );
                        const fallbackB = normalizeSortValue(
                            valueGetter(right, "id"),
                        );
                        result =
                            typeof fallbackA === "number" &&
                            typeof fallbackB === "number"
                                ? fallbackA - fallbackB
                                : String(fallbackA).localeCompare(
                                      String(fallbackB),
                                      "zh-CN",
                                      {
                                          numeric: true,
                                          sensitivity: "base",
                                      },
                                  );
                    }
                    return sortState.direction === "asc" ? result : -result;
                });
            }

            function updateSort(sortState, key) {
                if (sortState.key === key) {
                    sortState.direction =
                        sortState.direction === "asc" ? "desc" : "asc";
                } else {
                    sortState.key = key;
                    sortState.direction = "asc";
                }
            }

