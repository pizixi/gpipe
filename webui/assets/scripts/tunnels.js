            function updateEditTunnelPasswordToggle() {
                const input = document.getElementById("editTunnelPassword");
                const button = document.getElementById(
                    "editTunnelPasswordToggle",
                );
                if (!input || !button) {
                    return;
                }
                const visible = input.type === "text";
                const label = getText(
                    visible ? "hide_password" : "show_password",
                );
                button.innerHTML = renderPasswordEyeIcon(visible);
                button.setAttribute("aria-label", label);
                button.setAttribute("title", label);
            }

            function setEditTunnelPasswordVisibility(visible) {
                const input = document.getElementById("editTunnelPassword");
                if (!input) {
                    return;
                }
                input.type = visible ? "text" : "password";
                updateEditTunnelPasswordToggle();
            }

            function toggleEditTunnelPasswordVisibility() {
                const input = document.getElementById("editTunnelPassword");
                if (!input) {
                    return;
                }
                setEditTunnelPasswordVisibility(input.type !== "text");
            }

            const tunnelPlayerPickerFieldIds = [
                "newTunnelReceiver",
                "newTunnelSender",
                "editTunnelReceiver",
                "editTunnelSender",
            ];

            function getTunnelPlayerPicker(fieldId) {
                return document.querySelector(
                    `[data-player-combobox="${fieldId}"]`,
                );
            }

            function getTunnelPlayerPickerInput(fieldId) {
                return document.getElementById(fieldId + "Input");
            }

            function getTunnelPlayerPickerValueInput(fieldId) {
                return document.getElementById(fieldId);
            }

            function getTunnelPlayerPickerMenu(fieldId) {
                return document.getElementById(fieldId + "Menu");
            }

            function getTunnelPlayerPickerSearchInput(fieldId) {
                return document.getElementById(fieldId + "SearchInput");
            }

            function getTunnelPlayerPickerToggle(fieldId) {
                return document.querySelector(
                    `[data-player-combobox-toggle="${fieldId}"]`,
                );
            }

            function resolveTunnelPlayerPickerValue(fieldId) {
                const input = getTunnelPlayerPickerValueInput(fieldId);
                const value = Number.parseInt(
                    String(input?.value ?? "").trim(),
                    10,
                );
                return Number.isNaN(value) ? 0 : value;
            }

            function formatTunnelPlayerPickerLabel(id, remark) {
                const text = String(remark ?? "").trim();
                return text === "" ? String(id) : `${id} · ${text}`;
            }

            function resolveTunnelPlayerDisplayValue(value) {
                const numericValue = Number.parseInt(String(value).trim(), 10);
                if (Number.isNaN(numericValue) || numericValue === 0) {
                    return getText("server_side");
                }
                const player = getPlayerById(numericValue);
                if (!player) {
                    return String(numericValue);
                }
                return formatTunnelPlayerPickerLabel(player.id, player.remark);
            }

            function syncTunnelPlayerPickerToggleLabel(fieldId) {
                const toggle = getTunnelPlayerPickerToggle(fieldId);
                if (!toggle) {
                    return;
                }
                const label = getText("tunnel_player_picker_toggle");
                toggle.setAttribute("aria-label", label);
                toggle.setAttribute("title", label);
            }

            function syncTunnelPlayerPickerDisplay(fieldId) {
                const input = getTunnelPlayerPickerInput(fieldId);
                if (!input) {
                    return;
                }
                input.value = resolveTunnelPlayerDisplayValue(
                    resolveTunnelPlayerPickerValue(fieldId),
                );
            }

            function alignTunnelPlayerDropdownMenu(fieldId) {
                const picker = getTunnelPlayerPicker(fieldId);
                const menu = getTunnelPlayerPickerMenu(fieldId);
                const formGroup = picker?.closest(".form-group");
                if (!picker || !menu || !formGroup) {
                    return;
                }
                menu.style.left = `${-picker.offsetLeft}px`;
                menu.style.width = `${formGroup.clientWidth}px`;
            }

            function getFilteredTunnelPlayerChoices(keyword, selectedValue = 0) {
                const normalizedKeyword = String(keyword ?? "")
                    .trim()
                    .toLowerCase();
                const players = [...playersData]
                    .sort((left, right) => Number(left.id) - Number(right.id))
                    .filter((player) => {
                        if (normalizedKeyword === "") {
                            return true;
                        }
                        const idText = String(player.id);
                        const remarkText = String(player.remark ?? "")
                            .trim()
                            .toLowerCase();
                        return (
                            idText.includes(normalizedKeyword) ||
                            remarkText.includes(normalizedKeyword)
                        );
                    });
                const selectedIndex = players.findIndex(
                    (player) => Number(player.id) === Number(selectedValue),
                );
                if (selectedIndex > 0) {
                    const [selectedPlayer] = players.splice(selectedIndex, 1);
                    players.unshift(selectedPlayer);
                }
                return players;
            }

            function createTunnelPlayerOptionButton(
                fieldId,
                value,
                title,
                subtitle,
                options = {},
            ) {
                const button = document.createElement("button");
                button.type = "button";
                button.className = "player-combobox-option";
                button.dataset.value = String(value);
                if (options.isServer) {
                    button.classList.add("is-server");
                }
                if (options.isSelected) {
                    button.classList.add("is-selected");
                }
                const hoverTitle = String(subtitle ?? "").trim();
                if (hoverTitle !== "") {
                    button.title = hoverTitle;
                } else if (options.isServer) {
                    button.title = getText("tunnel_player_picker_service_hint");
                }
                button.setAttribute(
                    "aria-label",
                    subtitle
                        ? formatTunnelPlayerPickerLabel(title, subtitle)
                        : String(title),
                );
                const statusLabel = getText(
                    options.isOnline ? "online" : "offline",
                );
                const metaHTML = options.isServer
                    ? `<span class="player-combobox-option-badge is-server">${escapeHTML(
                          options.badgeText ||
                              getText("tunnel_player_picker_default_tag"),
                      )}</span>`
                    : `<span class="player-combobox-option-status ${options.isOnline ? "is-online" : "is-offline"}" aria-label="${escapeHTML(
                          statusLabel,
                      )}" title="${escapeHTML(statusLabel)}"></span>`;
                button.innerHTML = `
                    <span class="player-combobox-option-main">
                        <span class="player-combobox-option-id">${escapeHTML(title)}</span>
                        ${
                            subtitle
                                ? `<span class="player-combobox-option-remark" title="${escapeHTML(subtitle)}">${escapeHTML(subtitle)}</span>`
                                : ""
                        }
                    </span>
                    ${metaHTML}
                `;
                button.addEventListener("mouseenter", () => {
                    const buttons = getTunnelPlayerDropdownButtons(fieldId);
                    const index = buttons.indexOf(button);
                    if (index >= 0) {
                        setTunnelPlayerHighlight(fieldId, index);
                    }
                });
                button.addEventListener("mousedown", (event) => {
                    event.preventDefault();
                    applyTunnelPlayerSelection(fieldId, value);
                    closeTunnelPlayerDropdown(fieldId);
                });
                return button;
            }

            function getTunnelPlayerDropdownButtons(fieldId) {
                const menu = getTunnelPlayerPickerMenu(fieldId);
                return menu
                    ? Array.from(
                          menu.querySelectorAll(".player-combobox-option"),
                      )
                    : [];
            }

            function setTunnelPlayerHighlight(fieldId, index) {
                const picker = getTunnelPlayerPicker(fieldId);
                const buttons = getTunnelPlayerDropdownButtons(fieldId);
                if (!picker || buttons.length === 0) {
                    return;
                }
                const nextIndex = Math.max(
                    0,
                    Math.min(index, buttons.length - 1),
                );
                picker.dataset.highlightIndex = String(nextIndex);
                buttons.forEach((button, buttonIndex) => {
                    button.classList.toggle(
                        "is-highlighted",
                        buttonIndex === nextIndex,
                    );
                });
                buttons[nextIndex].scrollIntoView({
                    block: "nearest",
                });
            }

            function renderTunnelPlayerDropdown(fieldId, keepSearchFocus = false) {
                const picker = getTunnelPlayerPicker(fieldId);
                const menu = getTunnelPlayerPickerMenu(fieldId);
                if (!picker || !menu) {
                    return;
                }
                const selectedValue = resolveTunnelPlayerPickerValue(fieldId);
                const searchKeyword = String(
                    picker.dataset.searchKeyword ?? "",
                ).trim();
                const players = getFilteredTunnelPlayerChoices(
                    searchKeyword,
                    selectedValue,
                );
                menu.innerHTML = "";

                const fragment = document.createDocumentFragment();
                const searchWrap = document.createElement("div");
                searchWrap.className = "player-combobox-search";
                const searchInput = document.createElement("input");
                searchInput.type = "text";
                searchInput.id = fieldId + "SearchInput";
                searchInput.className = "player-combobox-search-input";
                searchInput.placeholder = getText(
                    "tunnel_player_picker_placeholder",
                );
                searchInput.autocomplete = "off";
                searchInput.spellcheck = false;
                searchInput.value = picker.dataset.searchKeyword ?? "";
                searchInput.addEventListener("input", () => {
                    picker.dataset.searchKeyword = searchInput.value;
                    if (picker.dataset.searchComposing === "true") {
                        return;
                    }
                    renderTunnelPlayerDropdown(fieldId, true);
                });
                searchInput.addEventListener("compositionstart", () => {
                    picker.dataset.searchComposing = "true";
                });
                searchInput.addEventListener("compositionend", () => {
                    picker.dataset.searchComposing = "false";
                    picker.dataset.searchKeyword = searchInput.value;
                    renderTunnelPlayerDropdown(fieldId, true);
                });
                searchInput.addEventListener("keydown", (event) => {
                    handleTunnelPlayerKeydown(event, fieldId);
                });
                searchInput.addEventListener("blur", () => {
                    handleTunnelPlayerBlur(fieldId);
                });
                searchWrap.appendChild(searchInput);
                fragment.appendChild(searchWrap);

                const optionsWrap = document.createElement("div");
                optionsWrap.className = "player-combobox-options";
                const selectedPlayerIndex = players.findIndex(
                    (player) => Number(player.id) === selectedValue,
                );
                const selectedPlayer =
                    selectedPlayerIndex >= 0
                        ? players[selectedPlayerIndex]
                        : null;
                const remainingPlayers =
                    selectedPlayerIndex >= 0
                        ? players.filter(
                              (player) =>
                                  Number(player.id) !== Number(selectedValue),
                          )
                        : players;

                if (selectedPlayer) {
                    optionsWrap.appendChild(
                        createTunnelPlayerOptionButton(
                            fieldId,
                            Number(selectedPlayer.id),
                            String(selectedPlayer.id),
                            getDisplayText(selectedPlayer.remark),
                            {
                                isOnline: !!selectedPlayer.online,
                                isSelected: true,
                            },
                        ),
                    );
                }

                optionsWrap.appendChild(
                    createTunnelPlayerOptionButton(
                        fieldId,
                        0,
                        "0",
                        getText("server_side"),
                        {
                            isServer: true,
                            isSelected: selectedValue === 0,
                            badgeText: getText(
                                "tunnel_player_picker_default_tag",
                            ),
                        },
                    ),
                );

                remainingPlayers.forEach((player) => {
                    optionsWrap.appendChild(
                        createTunnelPlayerOptionButton(
                            fieldId,
                            Number(player.id),
                            String(player.id),
                            getDisplayText(player.remark),
                            {
                                isOnline: !!player.online,
                                isSelected:
                                    Number(player.id) === selectedValue,
                            },
                        ),
                    );
                });

                if (players.length === 0) {
                    const emptyState = document.createElement("div");
                    emptyState.className = "player-combobox-empty";
                    emptyState.textContent = getText(
                        "tunnel_player_picker_empty",
                    );
                    optionsWrap.appendChild(emptyState);
                }

                const footer = document.createElement("div");
                footer.className = "player-combobox-footer";
                footer.textContent = getText(
                    "tunnel_player_picker_manual_hint",
                );
                optionsWrap.appendChild(footer);
                fragment.appendChild(optionsWrap);
                menu.appendChild(fragment);

                const buttons = getTunnelPlayerDropdownButtons(fieldId);
                const selectedIndex = buttons.findIndex(
                    (button) =>
                        Number.parseInt(button.dataset.value, 10) ===
                        selectedValue,
                );
                setTunnelPlayerHighlight(
                    fieldId,
                    selectedIndex >= 0 ? selectedIndex : 0,
                );
                syncTunnelPlayerPickerToggleLabel(fieldId);
                alignTunnelPlayerDropdownMenu(fieldId);
                if (keepSearchFocus) {
                    const nextSearchInput =
                        getTunnelPlayerPickerSearchInput(fieldId);
                    if (nextSearchInput) {
                        nextSearchInput.focus();
                        const value = String(nextSearchInput.value ?? "");
                        nextSearchInput.setSelectionRange(
                            value.length,
                            value.length,
                        );
                    }
                }
            }

            function openTunnelPlayerDropdown(fieldId, resetSearch = true) {
                const picker = getTunnelPlayerPicker(fieldId);
                if (!picker) {
                    return;
                }
                closeAllTunnelPlayerDropdowns(fieldId);
                picker.classList.add("is-open");
                if (resetSearch || !("searchKeyword" in picker.dataset)) {
                    picker.dataset.searchKeyword = "";
                }
                picker.dataset.searchComposing = "false";
                renderTunnelPlayerDropdown(fieldId, true);
            }

            function handleTunnelPlayerFocus(fieldId) {
                openTunnelPlayerDropdown(fieldId);
            }

            function closeTunnelPlayerDropdown(fieldId) {
                const picker = getTunnelPlayerPicker(fieldId);
                if (!picker) {
                    return;
                }
                picker.classList.remove("is-open");
                delete picker.dataset.highlightIndex;
                delete picker.dataset.searchKeyword;
                delete picker.dataset.searchComposing;
            }

            function closeAllTunnelPlayerDropdowns(exceptFieldId = null) {
                tunnelPlayerPickerFieldIds.forEach((fieldId) => {
                    if (fieldId !== exceptFieldId) {
                        closeTunnelPlayerDropdown(fieldId);
                    }
                });
            }

            function toggleTunnelPlayerDropdown(fieldId) {
                const picker = getTunnelPlayerPicker(fieldId);
                const input = getTunnelPlayerPickerInput(fieldId);
                if (!picker || !input) {
                    return;
                }
                if (picker.classList.contains("is-open")) {
                    closeTunnelPlayerDropdown(fieldId);
                    return;
                }
                openTunnelPlayerDropdown(fieldId);
            }

            function applyTunnelPlayerSelection(fieldId, value) {
                const input = getTunnelPlayerPickerValueInput(fieldId);
                if (!input) {
                    return;
                }
                const numericValue = Number.parseInt(String(value).trim(), 10);
                input.value = Number.isNaN(numericValue)
                    ? "0"
                    : String(numericValue);
                syncTunnelPlayerPickerDisplay(fieldId);
                if (getTunnelPlayerPicker(fieldId)?.classList.contains(
                    "is-open",
                )) {
                    renderTunnelPlayerDropdown(fieldId);
                }
            }

            function handleTunnelPlayerBlur(fieldId) {
                window.setTimeout(() => {
                    const picker = getTunnelPlayerPicker(fieldId);
                    if (
                        !picker ||
                        !picker.classList.contains("is-open") ||
                        picker.contains(document.activeElement)
                    ) {
                        return;
                    }
                    closeTunnelPlayerDropdown(fieldId);
                }, 120);
            }

            function handleTunnelPlayerKeydown(event, fieldId) {
                if (event.isComposing || event.keyCode === 229) {
                    return;
                }
                const picker = getTunnelPlayerPicker(fieldId);
                const currentIndex = Number.parseInt(
                    String(picker?.dataset.highlightIndex ?? "0"),
                    10,
                );
                switch (event.key) {
                    case "ArrowDown":
                        event.preventDefault();
                        if (!picker?.classList.contains("is-open")) {
                            openTunnelPlayerDropdown(fieldId);
                        }
                        {
                            const buttons = getTunnelPlayerDropdownButtons(
                                fieldId,
                            );
                        if (buttons.length > 0) {
                            setTunnelPlayerHighlight(
                                fieldId,
                                (currentIndex + 1) % buttons.length,
                            );
                        }
                        }
                        break;
                    case "ArrowUp":
                        event.preventDefault();
                        if (!picker?.classList.contains("is-open")) {
                            openTunnelPlayerDropdown(fieldId);
                        }
                        {
                            const buttons = getTunnelPlayerDropdownButtons(
                                fieldId,
                            );
                        if (buttons.length > 0) {
                            setTunnelPlayerHighlight(
                                fieldId,
                                (currentIndex - 1 + buttons.length) %
                                    buttons.length,
                            );
                        }
                        }
                        break;
                    case "Enter":
                        event.preventDefault();
                        if (!picker?.classList.contains("is-open")) {
                            openTunnelPlayerDropdown(fieldId);
                            return;
                        }
                        {
                            const buttons = getTunnelPlayerDropdownButtons(
                                fieldId,
                            );
                        if (
                            picker?.classList.contains("is-open") &&
                            buttons.length > 0
                        ) {
                            const targetButton =
                                buttons[
                                    Math.max(
                                        0,
                                        Math.min(
                                            currentIndex,
                                            buttons.length - 1,
                                        ),
                                    )
                                ];
                            if (targetButton) {
                                applyTunnelPlayerSelection(
                                    fieldId,
                                    targetButton.dataset.value,
                                );
                                closeTunnelPlayerDropdown(fieldId);
                                return;
                            }
                        }
                        }
                        break;
                    case "Escape":
                        event.preventDefault();
                        syncTunnelPlayerPickerDisplay(fieldId);
                        closeTunnelPlayerDropdown(fieldId);
                        break;
                    default:
                        break;
                }
            }

            function refreshTunnelPlayerPickers() {
                tunnelPlayerPickerFieldIds.forEach((fieldId) => {
                    syncTunnelPlayerPickerToggleLabel(fieldId);
                    const picker = getTunnelPlayerPicker(fieldId);
                    const input = getTunnelPlayerPickerInput(fieldId);
                    if (!picker || !input) {
                        return;
                    }
                    if (
                        picker.classList.contains("is-open")
                    ) {
                        renderTunnelPlayerDropdown(
                            fieldId,
                            document.activeElement ===
                                getTunnelPlayerPickerSearchInput(fieldId),
                        );
                        return;
                    }
                    syncTunnelPlayerPickerDisplay(fieldId);
                });
            }

            window.addEventListener("resize", () => {
                tunnelPlayerPickerFieldIds.forEach((fieldId) => {
                    if (
                        getTunnelPlayerPicker(fieldId)?.classList.contains(
                            "is-open",
                        )
                    ) {
                        alignTunnelPlayerDropdownMenu(fieldId);
                    }
                });
            });

            document.addEventListener("click", (event) => {
                if (event.target.closest(".player-combobox")) {
                    return;
                }
                closeAllTunnelPlayerDropdowns();
            });

            function isDynamicTargetTunnelType(tunnelType) {
                return tunnelType === 2 || tunnelType === 3 || tunnelType === 4;
            }

            function isUsernameTunnelType(tunnelType) {
                return tunnelType === 2 || tunnelType === 3;
            }

            function isPasswordTunnelType(tunnelType) {
                return tunnelType === 2 || tunnelType === 3 || tunnelType === 4;
            }

            function isShadowsocksTunnelType(tunnelType) {
                return tunnelType === 4;
            }

            function syncTunnelEncryptionOptions(
                prefix,
                tunnelType,
                selectedValue = null,
            ) {
                const select = document.getElementById(
                    prefix + "TunnelEncryption",
                );
                const currentValue = selectedValue ?? select.value;
                const options = isShadowsocksTunnelType(tunnelType)
                    ? tunnelShadowsocksMethodOptions
                    : tunnelTransportEncryptionOptions.map((option) => ({
                          value: option.value,
                          label: getText(option.labelKey),
                      }));
                const fallbackValue = isShadowsocksTunnelType(tunnelType)
                    ? defaultShadowsocksMethod
                    : "None";

                select.innerHTML = "";
                options.forEach((option) => {
                    const element = document.createElement("option");
                    element.value = option.value;
                    element.textContent = option.label;
                    select.appendChild(element);
                });
                select.value = options.some(
                    (option) => option.value === currentValue,
                )
                    ? currentValue
                    : fallbackValue;
            }


            function tunnelSortValue(tunnel, key) {
                switch (key) {
                    case "receiver":
                        return tunnel.receiver;
                    case "source":
                        return tunnel.source;
                    case "sender":
                        return tunnel.sender;
                    case "endpoint":
                        return tunnel.endpoint;
                    case "tunnel_type":
                        return tunnel.tunnel_type;
                    case "enabled":
                        return !!tunnel.enabled;
                    case "is_compressed":
                        return !!tunnel.is_compressed;
                    case "description":
                        return tunnel.description;
                    default:
                        return tunnel.id;
                }
            }


            function getTunnelSearchKeyword() {
                const input = document.getElementById("tunnelPlayerIdSearch");
                return input ? input.value.trim() : "";
            }

            function renderTunnels() {
                const table = document.getElementById("tunnelTable");
                table.innerHTML = `<tr>
                    ${renderSortableHeader("receiver_id", tunnelSort, "receiver", "toggleTunnelSort")}
                    ${renderSortableHeader("source", tunnelSort, "source", "toggleTunnelSort")}
                    ${renderSortableHeader("sender_id", tunnelSort, "sender", "toggleTunnelSort")}
                    ${renderSortableHeader("target_tab_title", tunnelSort, "endpoint", "toggleTunnelSort")}
                    ${renderSortableHeader("protocol_type", tunnelSort, "tunnel_type", "toggleTunnelSort")}
                    ${renderSortableHeader("status", tunnelSort, "enabled", "toggleTunnelSort")}
                    ${renderSortableHeader("description", tunnelSort, "description", "toggleTunnelSort")}
                    <th data-i18n="actions">${getText("actions")}</th>
                </tr>`;

                const keyword = getTunnelSearchKeyword().toLowerCase();
                const numericPlayerID =
                    keyword !== "" && /^\d+$/.test(keyword)
                        ? Number.parseInt(keyword, 10)
                        : null;
                const filtered = tunnelsData.filter((tunnel) => {
                    if (keyword === "") {
                        return true;
                    }
                    const description = String(tunnel.description ?? "")
                        .toLowerCase()
                        .trim();
                    const matchesPlayerID =
                        numericPlayerID !== null &&
                        (Number(tunnel.sender) === numericPlayerID ||
                            Number(tunnel.receiver) === numericPlayerID);
                    return matchesPlayerID || description.includes(keyword);
                });
                const tunnels = sortCollection(
                    filtered,
                    tunnelSort,
                    tunnelSortValue,
                );
                if (tunnels.length === 0) {
                    table.innerHTML += renderEmptyState(8, "empty_tunnels");
                    return;
                }

                tunnels.forEach((tunnel) => {
                    const row = table.insertRow();
                    const payload = encodePayload(tunnel);
                    const ingressText =
                        tunnel.receiver === 0
                            ? getText("server_side")
                            : String(tunnel.receiver);
                    const egressText =
                        tunnel.sender === 0
                            ? getText("server_side")
                            : String(tunnel.sender);
                    row.innerHTML = `
                        <td data-label="${getText("receiver_id")}">
                            <span class="id-chip">${escapeHTML(ingressText)}</span>
                        </td>
                        <td data-label="${getText("source")}">
                            <span class="cell-ellipsis mono">${formatPlainValue(
                                tunnel.source,
                            )}</span>
                        </td>
                        <td data-label="${getText("sender_id")}">
                            <span class="id-chip">${escapeHTML(egressText)}</span>
                        </td>
                        <td data-label="${getText("target_tab_title")}">
                            <span class="cell-ellipsis mono">${formatPlainValue(
                                tunnel.endpoint,
                            )}</span>
                        </td>
                        <td data-label="${getText("protocol_type")}">${renderProtocolChip(
                            tunnel.tunnel_type,
                        )}</td>
                        <td data-label="${getText("status")}">${renderStatusBadge(
                            !!tunnel.enabled,
                            "enabled",
                            "disabled",
                        )}</td>
                        <td data-label="${getText("description")}">
                            <span class="cell-ellipsis">${formatPlainValue(
                                tunnel.description,
                            )}</span>
                        </td>
                        <td data-label="${getText("actions")}">
                            <div class="action-buttons">
                                ${renderActionButton(
                                    "btn-primary",
                                    getText("edit_tunnel"),
                                    `showEditTunnelModalFromPayload('${payload}')`,
                                    "edit",
                                )}
                                ${renderActionButton(
                                    "btn-danger",
                                    getText("delete_button"),
                                    `removeTunnel(${escapeHTML(tunnel.id)})`,
                                    "delete",
                                )}
                                ${renderActionButton(
                                    tunnel.enabled
                                        ? "btn-warning"
                                        : "btn-success",
                                    getText(
                                        tunnel.enabled ? "disabled" : "enabled",
                                    ),
                                    `toggleTunnelStatus(${escapeHTML(
                                        tunnel.id,
                                    )}, ${!!tunnel.enabled})`,
                                    tunnel.enabled ? "disable" : "enable",
                                )}
                            </div>
                        </td>`;
                });
            }

            async function loadTunnels() {
                try {
                    const data = await apiRequest("/api/tunnel_list", "POST", {
                        page_number: 0,
                        page_size: 0,
                    });
                    if (Number(data?.code) === 10086) {
                        showLoginForm();
                        return;
                    }
                    tunnelsData = Array.isArray(data.tunnels)
                        ? data.tunnels
                        : [];
                    renderTunnels();
                } catch (error) {
                    console.error(error);
                }
            }


            function scheduleTunnelSearch() {
                clearTimeout(tunnelSearchTimer);
                tunnelSearchTimer = setTimeout(() => {
                    renderTunnels();
                }, 180);
            }

            function parseTunnelNumber(id) {
                const value = Number.parseInt(
                    document.getElementById(id).value.trim(),
                    10,
                );
                return Number.isNaN(value) ? 0 : value;
            }

            function readTunnelForm(prefix) {
                const tunnelType = Number.parseInt(
                    document.getElementById(prefix + "TunnelType").value,
                    10,
                );
                const usesDynamicTarget = isDynamicTargetTunnelType(tunnelType);
                const usesUsername = isUsernameTunnelType(tunnelType);
                const usesPassword = isPasswordTunnelType(tunnelType);
                const encryptionValue = document.getElementById(
                    prefix + "TunnelEncryption",
                ).value;
                return {
                    source: document
                        .getElementById(prefix + "TunnelSource")
                        .value.trim(),
                    endpoint: usesDynamicTarget
                        ? ""
                        : document
                              .getElementById(prefix + "TunnelEndpoint")
                              .value.trim(),
                    enabled: 1,
                    sender: parseTunnelNumber(prefix + "TunnelSender"),
                    receiver: parseTunnelNumber(prefix + "TunnelReceiver"),
                    description: document
                        .getElementById(prefix + "TunnelDescription")
                        .value.trim(),
                    tunnel_type: Number.isNaN(tunnelType) ? 0 : tunnelType,
                    password: usesPassword
                        ? document
                              .getElementById(prefix + "TunnelPassword")
                              .value.trim()
                        : "",
                    username: usesUsername
                        ? document
                              .getElementById(prefix + "TunnelUsername")
                              .value.trim()
                        : "",
                    is_compressed: document.getElementById(
                        prefix + "TunnelCompressed",
                    ).checked
                        ? 1
                        : 0,
                    encryption_method:
                        encryptionValue ||
                        (isShadowsocksTunnelType(tunnelType)
                            ? defaultShadowsocksMethod
                            : "None"),
                    custom_mapping: {},
                };
            }

            function validateTunnelForm(prefix) {
                const tunnelType = Number.parseInt(
                    document.getElementById(prefix + "TunnelType").value,
                    10,
                );
                const passwordInput = document.getElementById(
                    prefix + "TunnelPassword",
                );
                if (
                    isShadowsocksTunnelType(tunnelType) &&
                    passwordInput.value.trim() === ""
                ) {
                    alert(getText("shadowsocks_password_required"));
                    passwordInput.focus();
                    return false;
                }
                return true;
            }


            function resetTunnelForm(prefix) {
                document.getElementById(prefix + "TunnelType").value = "0";
                document.getElementById(prefix + "TunnelSource").value = "";
                document.getElementById(prefix + "TunnelEndpoint").value = "";
                document.getElementById(prefix + "TunnelSender").value = "0";
                document.getElementById(prefix + "TunnelReceiver").value = "0";
                document.getElementById(prefix + "TunnelDescription").value =
                    "";
                document.getElementById(prefix + "TunnelPassword").value = "";
                document.getElementById(prefix + "TunnelUsername").value = "";
                document.getElementById(prefix + "TunnelCompressed").checked =
                    true;
                closeAllTunnelPlayerDropdowns();
                refreshTunnelPlayerPickers();
                toggleTunnelTypeFields(prefix, "None");
            }

            function showAddTunnelModal() {
                resetTunnelForm("new");
                refreshTunnelPlayerPickers();
                document.getElementById("addTunnelModal").style.display =
                    "block";
            }

            function showEditTunnelModal(tunnel) {
                currentEditingTunnelId = tunnel.id;
                document.getElementById("editTunnelSource").value =
                    tunnel.source || "";
                document.getElementById("editTunnelEndpoint").value =
                    tunnel.endpoint || "";
                document.getElementById("editTunnelSender").value =
                    tunnel.sender ?? 0;
                document.getElementById("editTunnelReceiver").value =
                    tunnel.receiver ?? 0;
                document.getElementById("editTunnelDescription").value =
                    tunnel.description || "";
                document.getElementById("editTunnelType").value = String(
                    tunnel.tunnel_type ?? 0,
                );
                document.getElementById("editTunnelPassword").value =
                    tunnel.password || "";
                setEditTunnelPasswordVisibility(false);
                document.getElementById("editTunnelUsername").value =
                    tunnel.username || "";
                document.getElementById("editTunnelCompressed").checked =
                    !!tunnel.is_compressed;
                closeAllTunnelPlayerDropdowns();
                refreshTunnelPlayerPickers();
                toggleTunnelTypeFields(
                    "edit",
                    tunnel.encryption_method ||
                        (isShadowsocksTunnelType(tunnel.tunnel_type ?? 0)
                            ? defaultShadowsocksMethod
                            : "None"),
                );
                document.getElementById("editTunnelModal").style.display =
                    "block";
            }

            function showEditTunnelModalFromPayload(payload) {
                const tunnel = decodePayload(payload);
                if (!tunnel) {
                    return;
                }
                showEditTunnelModal(tunnel);
            }


            async function addTunnel() {
                if (!validateTunnelForm("new")) {
                    return;
                }
                try {
                    const data = await apiRequest(
                        "/api/add_tunnel",
                        "POST",
                        readTunnelForm("new"),
                    );
                    if (data.code === 0) {
                        closeModal("addTunnelModal");
                        loadTunnels();
                    } else {
                        alert(getText("add_tunnel_failed") + data.msg);
                    }
                } catch (error) {
                    console.error(error);
                }
            }

            async function updateTunnel() {
                if (!validateTunnelForm("edit")) {
                    return;
                }
                try {
                    const data = await apiRequest(
                        "/api/update_tunnel",
                        "POST",
                        {
                            id: currentEditingTunnelId,
                            ...readTunnelForm("edit"),
                        },
                    );
                    if (data.code === 0) {
                        closeModal("editTunnelModal");
                        loadTunnels();
                    } else {
                        alert(getText("update_tunnel_failed") + data.msg);
                    }
                } catch (error) {
                    console.error(error);
                }
            }

            async function removeTunnel(id) {
                if (!confirm(getText("confirm_delete_tunnel"))) {
                    return;
                }
                try {
                    const data = await apiRequest(
                        "/api/remove_tunnel",
                        "POST",
                        {
                            id,
                        },
                    );
                    if (data.code === 0) {
                        loadTunnels();
                    } else {
                        alert(getText("delete_tunnel_failed") + data.msg);
                    }
                } catch (error) {
                    console.error(error);
                }
            }


            function toggleTunnelTypeFields(prefix, selectedEncryption = null) {
                const type = Number.parseInt(
                    document.getElementById(prefix + "TunnelType").value,
                    10,
                );
                const tunnelType = Number.isNaN(type) ? 0 : type;
                const typeInput = document.getElementById(
                    prefix + "TunnelType",
                );
                const endpointGroup = document.getElementById(
                    prefix + "TunnelEndpointGroup",
                );
                const usernameGroup = document.getElementById(
                    prefix + "TunnelUsernameGroup",
                );
                const passwordGroup = document.getElementById(
                    prefix + "TunnelPasswordGroup",
                );
                const endpointInput = document.getElementById(
                    prefix + "TunnelEndpoint",
                );
                const usernameInput = document.getElementById(
                    prefix + "TunnelUsername",
                );
                const passwordInput = document.getElementById(
                    prefix + "TunnelPassword",
                );
                const usesDynamicTarget = isDynamicTargetTunnelType(tunnelType);
                const usesUsername = isUsernameTunnelType(tunnelType);
                const usesPassword = isPasswordTunnelType(tunnelType);

                if (typeInput) {
                    syncTunnelEncryptionOptions(
                        prefix,
                        tunnelType,
                        selectedEncryption,
                    );
                }

                endpointGroup.style.display = usesDynamicTarget
                    ? "none"
                    : "flex";
                usernameGroup.style.display = usesUsername ? "flex" : "none";
                passwordGroup.style.display = usesPassword ? "flex" : "none";
                if (usesDynamicTarget) {
                    endpointInput.value = "";
                }
                if (!usesUsername) {
                    usernameInput.value = "";
                }
                if (!usesPassword) {
                    passwordInput.value = "";
                }
                passwordInput.required = isShadowsocksTunnelType(tunnelType);
            }

            async function toggleTunnelStatus(id, currentStatus) {
                try {
                    const data = await apiRequest("/api/tunnel_list", "POST", {
                        page_number: 0,
                        page_size: 0,
                    });
                    const tunnel = Array.isArray(data.tunnels)
                        ? data.tunnels.find((item) => item.id === id)
                        : null;
                    if (!tunnel) {
                        throw new Error(getText("tunnel_not_found"));
                    }
                    tunnel.enabled = !currentStatus ? 1 : 0;
                    tunnel.is_compressed = tunnel.is_compressed ? 1 : 0;
                    const result = await apiRequest(
                        "/api/update_tunnel",
                        "POST",
                        tunnel,
                    );
                    if (result.code === 0) {
                        loadTunnels();
                    } else {
                        alert(getText("update_tunnel_failed") + result.msg);
                    }
                } catch (error) {
                    console.error(error);
                }
            }
