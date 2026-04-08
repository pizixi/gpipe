            function getPlayerById(id) {
                return (
                    playersData.find(
                        (player) => Number(player.id) === Number(id),
                    ) || null
                );
            }

            function isPlayerOnline(id) {
                return !!getPlayerById(id)?.online;
            }


            function generateRandomPlayerKey() {
                const bytes = new Uint8Array(playerKeyLength);
                if (window.crypto?.getRandomValues) {
                    window.crypto.getRandomValues(bytes);
                } else {
                    for (let i = 0; i < bytes.length; i += 1) {
                        bytes[i] = Math.floor(Math.random() * 256);
                    }
                }
                return Array.from(
                    bytes,
                    (byte) =>
                        playerKeyCharacters[byte % playerKeyCharacters.length],
                ).join("");
            }

            function generatePlayerKey(inputId) {
                const input = document.getElementById(inputId);
                if (!input || input.readOnly || input.disabled) {
                    return;
                }
                input.value = generateRandomPlayerKey();
                input.focus();
                input.select();
            }


            function playerSortValue(player, key) {
                switch (key) {
                    case "id":
                        return player.id;
                    case "remark":
                        return player.remark;
                    case "key":
                        return player.key;
                    case "create_time":
                        return Date.parse(player.create_time || "") || 0;
                    case "online":
                        return !!player.online;
                    default:
                        return player.id;
                }
            }


            function togglePlayerSort(key) {
                updateSort(playerSort, key);
                renderPlayers();
            }


            function renderPlayers() {
                const table = document.getElementById("playerTable");
                table.innerHTML = `<tr>
                    ${renderSortableHeader("id", playerSort, "id", "togglePlayerSort")}
                    ${renderSortableHeader("player_remark", playerSort, "remark", "togglePlayerSort")}
                    ${renderSortableHeader("player_key", playerSort, "key", "togglePlayerSort")}
                    ${renderSortableHeader("create_time", playerSort, "create_time", "togglePlayerSort")}
                    ${renderSortableHeader("online_status", playerSort, "online", "togglePlayerSort")}
                    <th data-i18n="actions">${getText("actions")}</th>
                </tr>`;

                const keyword = document
                    .getElementById("playerSearch")
                    .value.trim()
                    .toLowerCase();
                const filtered = playersData.filter((player) => {
                    if (keyword === "") {
                        return true;
                    }
                    return String(player.remark ?? "")
                        .toLowerCase()
                        .includes(keyword);
                });
                const players = sortCollection(
                    filtered,
                    playerSort,
                    playerSortValue,
                );

                if (players.length === 0) {
                    table.innerHTML += renderEmptyState(6, "empty_players");
                    return;
                }

                players.forEach((player) => {
                    const row = table.insertRow();
                    const payload = encodePayload(player);
                    const isOnline = !!player.online;
                    row.innerHTML = `
                        <td data-label="${getText("id")}">${escapeHTML(
                            player.id,
                        )}</td>
                        <td data-label="${getText("player_remark")}">
                            <span class="cell-ellipsis">${formatPlainValue(
                                player.remark,
                            )}</span>
                        </td>
                        <td data-label="${getText("player_key")}">
                            <span class="cell-ellipsis">${formatPlainValue(
                                player.key,
                            )}</span>
                        </td>
                        <td data-label="${getText("create_time")}">
                            <span class="cell-ellipsis">${formatDateTimeValue(
                                player.create_time,
                            )}</span>
                        </td>
                        <td data-label="${getText("online_status")}">${renderOnlineBadge(
                            !!player.online,
                        )}</td>
                        <td data-label="${getText("actions")}">
                            <div class="action-buttons compact-actions">
                                ${renderActionButton(
                                    "btn-primary",
                                    getText("edit_player"),
                                    `showEditPlayerModalFromPayload('${payload}')`,
                                    "edit",
                                )}
                                ${renderActionButton(
                                    "btn-accent",
                                    getText("generate_client"),
                                    `showGenerateClientModal(${escapeHTML(
                                        player.id,
                                    )})`,
                                    "download",
                                )}
                                ${renderActionButton(
                                    "btn-danger",
                                    isOnline
                                        ? getText(
                                              "online_player_delete_forbidden",
                                          )
                                        : getText("delete_button"),
                                    `removePlayer(${escapeHTML(player.id)})`,
                                    "delete",
                                    isOnline,
                                )}
                            </div>
                        </td>`;
                });
            }

            async function loadPlayers() {
                try {
                    const data = await apiRequest("/api/player_list", "POST", {
                        page_number: 0,
                        page_size: 0,
                    });
                    if (Number(data?.code) === 10086) {
                        showLoginForm();
                        return;
                    }
                    playersData = Array.isArray(data.players)
                        ? data.players
                        : [];
                    if (currentEditingPlayerId !== null) {
                        setEditPlayerKeyLocked(
                            isPlayerOnline(currentEditingPlayerId),
                        );
                    }
                    renderPlayers();
                    if (typeof refreshTunnelPlayerPickers === "function") {
                        refreshTunnelPlayerPickers();
                    }
                } catch (error) {
                    console.error(error);
                }
            }


            function searchPlayers() {
                renderPlayers();
            }

            function schedulePlayerSearch() {
                clearTimeout(playerSearchTimer);
                playerSearchTimer = setTimeout(() => {
                    renderPlayers();
                }, 180);
            }


            function showAddPlayerModal() {
                document.getElementById("newPlayerRemark").value = "";
                document.getElementById("newPlayerKey").value = "";
                document.getElementById("addPlayerModal").style.display =
                    "block";
            }

            function setEditPlayerKeyLocked(locked) {
                const keyInput = document.getElementById("editPlayerKey");
                const generateButton = document.querySelector(
                    "#editPlayerModal [data-generate-key-button]",
                );
                const hint = document.getElementById("editPlayerKeyHint");
                if (!keyInput || !generateButton || !hint) {
                    return;
                }
                keyInput.readOnly = locked;
                generateButton.disabled = locked;
                hint.style.display = locked ? "block" : "none";
            }

            function showEditPlayerModal(id, remark, key, online = false) {
                currentEditingPlayerId = id;
                document.getElementById("editPlayerRemark").value =
                    remark || "";
                document.getElementById("editPlayerKey").value = key || "";
                setEditPlayerKeyLocked(!!online);
                document.getElementById("editPlayerModal").style.display =
                    "block";
            }

            function showEditPlayerModalFromPayload(payload) {
                const player = decodePayload(payload);
                if (!player) {
                    return;
                }
                showEditPlayerModal(
                    player.id,
                    player.remark,
                    player.key,
                    !!player.online,
                );
            }


            async function addPlayer() {
                const remark = document
                    .getElementById("newPlayerRemark")
                    .value.trim();
                const key = document
                    .getElementById("newPlayerKey")
                    .value.trim();
                try {
                    const data = await apiRequest("/api/add_player", "POST", {
                        remark,
                        key,
                    });
                    if (data.code === 0) {
                        closeModal("addPlayerModal");
                        loadPlayers();
                    } else {
                        alert(getText("add_player_failed") + data.msg);
                    }
                } catch (error) {
                    console.error(error);
                }
            }

            async function updatePlayer() {
                const id = currentEditingPlayerId;
                const remark = document
                    .getElementById("editPlayerRemark")
                    .value.trim();
                const key = document
                    .getElementById("editPlayerKey")
                    .value.trim();
                const player = getPlayerById(id);
                if (
                    player &&
                    !!player.online &&
                    key !== String(player.key ?? "").trim()
                ) {
                    alert(getText("online_player_key_locked"));
                    return;
                }
                try {
                    const data = await apiRequest(
                        "/api/update_player",
                        "POST",
                        {
                            id,
                            remark,
                            key,
                        },
                    );
                    if (data.code === 0) {
                        closeModal("editPlayerModal");
                        loadPlayers();
                    } else {
                        alert(getText("update_player_failed") + data.msg);
                    }
                } catch (error) {
                    console.error(error);
                }
            }

            async function removePlayer(id) {
                if (isPlayerOnline(id)) {
                    alert(getText("online_player_delete_forbidden"));
                    return;
                }
                if (!confirm(getText("confirm_delete_player"))) {
                    return;
                }
                try {
                    const data = await apiRequest(
                        "/api/remove_player",
                        "POST",
                        {
                            id,
                        },
                    );
                    if (data.code === 0) {
                        loadPlayers();
                    } else {
                        alert(getText("delete_player_failed") + data.msg);
                    }
                } catch (error) {
                    console.error(error);
                }
            }

