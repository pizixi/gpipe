            function createDefaultClientBuildSettings() {
                return {
                    server: "",
                    enable_tls: false,
                    tls_server_name: "",
                    use_shadowsocks: false,
                    ss_server: "",
                    ss_method: defaultShadowsocksMethod,
                    ss_password: "",
                };
            }


            function renderClientTargetOptions(selectedValue = null) {
                const select = document.getElementById("generateClientTarget");
                if (!select) {
                    return;
                }
                const currentValue = selectedValue ?? select.value;
                select.innerHTML = "";
                clientBuildTargetOptions.forEach((option) => {
                    const element = document.createElement("option");
                    element.value = option.value;
                    element.textContent =
                        option.label[currentLanguage] || option.label.zh;
                    select.appendChild(element);
                });
                select.value = clientBuildTargetOptions.some(
                    (option) => option.value === currentValue,
                )
                    ? currentValue
                    : clientBuildTargetOptions[0].value;
            }

            function fillClientBuildSettingsForm(settings) {
                const data = {
                    ...createDefaultClientBuildSettings(),
                    ...(settings || {}),
                };
                document.getElementById("clientServer").value =
                    data.server || "";
                document.getElementById("clientEnableTLS").checked =
                    !!data.enable_tls;
                document.getElementById("clientTLSServerName").value =
                    data.tls_server_name || "";
                document.getElementById("clientUseShadowsocks").checked =
                    !!data.use_shadowsocks;
                document.getElementById("clientSSServer").value =
                    data.ss_server || "";
                document.getElementById("clientSSMethod").value =
                    data.ss_method || defaultShadowsocksMethod;
                document.getElementById("clientSSPassword").value =
                    data.ss_password || "";
                toggleClientSettingsSSFields();
            }

            function readClientBuildSettingsForm() {
                return {
                    server: document
                        .getElementById("clientServer")
                        .value.trim(),
                    enable_tls:
                        document.getElementById("clientEnableTLS").checked,
                    tls_server_name: document
                        .getElementById("clientTLSServerName")
                        .value.trim(),
                    use_shadowsocks: document.getElementById(
                        "clientUseShadowsocks",
                    ).checked,
                    ss_server: document
                        .getElementById("clientSSServer")
                        .value.trim(),
                    ss_method:
                        document.getElementById("clientSSMethod").value ||
                        defaultShadowsocksMethod,
                    ss_password: document
                        .getElementById("clientSSPassword")
                        .value.trim(),
                };
            }

            function toggleClientSettingsSSFields() {
                const enabled = document.getElementById(
                    "clientUseShadowsocks",
                ).checked;
                [
                    "clientSSServerGroup",
                    "clientSSMethodGroup",
                    "clientSSPasswordGroup",
                ].forEach((id) => {
                    const element = document.getElementById(id);
                    if (element) {
                        element.style.display = enabled ? "flex" : "none";
                    }
                });
            }

            function renderClientSettingsSummary(settings) {
                const summary = document.getElementById(
                    "generateClientSettingsSummary",
                );
                if (!summary) {
                    return;
                }
                const data = {
                    ...createDefaultClientBuildSettings(),
                    ...(settings || {}),
                };
                const lines = [
                    `${getText("server_address")}: ${escapeHTML(
                        getDisplayText(data.server),
                    )}`,
                    `${getText("enable_tls")}: ${escapeHTML(
                        data.enable_tls
                            ? getText("enabled")
                            : getText("disabled"),
                    )}`,
                ];
                if ((data.tls_server_name || "").trim() !== "") {
                    lines.push(
                        `${getText("tls_server_name")}: ${escapeHTML(
                            data.tls_server_name,
                        )}`,
                    );
                }
                lines.push(
                    `${getText("use_shadowsocks")}: ${escapeHTML(
                        data.use_shadowsocks
                            ? getText("enabled")
                            : getText("disabled"),
                    )}`,
                );
                if (data.use_shadowsocks) {
                    lines.push(
                        `${getText("shadowsocks_server")}: ${escapeHTML(
                            getDisplayText(data.ss_server),
                        )}`,
                    );
                    lines.push(
                        `${getText("shadowsocks_method")}: ${escapeHTML(
                            getDisplayText(data.ss_method),
                        )}`,
                    );
                }
                summary.innerHTML = lines.join("<br />");
            }

            async function loadClientBuildSettings() {
                try {
                    const data = await apiRequest(
                        "/api/client_build_settings",
                        "POST",
                        {},
                    );
                    if (Number(data?.code) === 10086) {
                        showLoginForm();
                        return;
                    }
                    clientBuildSettingsData = {
                        ...createDefaultClientBuildSettings(),
                        ...(data.settings || {}),
                    };
                } catch (error) {
                    console.error(error);
                    clientBuildSettingsData =
                        createDefaultClientBuildSettings();
                }
                fillClientBuildSettingsForm(clientBuildSettingsData);
                renderClientSettingsSummary(clientBuildSettingsData);
                renderClientTargetOptions();
            }

            async function saveClientBuildSettings() {
                const settings = readClientBuildSettingsForm();
                try {
                    const data = await apiRequest(
                        "/api/update_client_build_settings",
                        "POST",
                        settings,
                    );
                    if (data.code === 0) {
                        clientBuildSettingsData = { ...settings };
                        fillClientBuildSettingsForm(clientBuildSettingsData);
                        renderClientSettingsSummary(clientBuildSettingsData);
                        alert(getText("settings_saved"));
                    } else {
                        alert(getText("save_settings_failed") + data.msg);
                    }
                } catch (error) {
                    console.error(error);
                }
            }

            function showSettingsTab() {
                const button = Array.from(
                    document.getElementsByClassName("tablinks"),
                ).find((item) =>
                    String(item.getAttribute("onclick") || "").includes(
                        "'Settings'",
                    ),
                );
                if (button) {
                    button.click();
                }
            }

            function showGenerateClientModal(id) {
                const player = getPlayerById(id);
                if (!player) {
                    alert(getText("player_not_found"));
                    return;
                }
                if (
                    !clientBuildSettingsData ||
                    String(clientBuildSettingsData.server || "").trim() === ""
                ) {
                    alert(getText("client_settings_required"));
                    showSettingsTab();
                    return;
                }
                currentGeneratingPlayerId = id;
                document.getElementById("generateClientPlayerId").textContent =
                    String(player.id);
                document.getElementById(
                    "generateClientPlayerRemark",
                ).textContent = getDisplayText(player.remark);
                renderClientTargetOptions();
                renderClientSettingsSummary(clientBuildSettingsData);
                document.getElementById("generateClientModal").style.display =
                    "block";
            }

            function parseDownloadFilename(disposition) {
                const value = String(disposition || "");
                const utf8Match = value.match(/filename\*=UTF-8''([^;]+)/i);
                if (utf8Match && utf8Match[1]) {
                    return decodeURIComponent(utf8Match[1]);
                }
                const match = value.match(/filename="([^"]+)"/i);
                return match && match[1] ? match[1] : "";
            }

            async function downloadGeneratedClient() {
                if (currentGeneratingPlayerId === null) {
                    return;
                }
                const target = document.getElementById(
                    "generateClientTarget",
                ).value;
                try {
                    const response = await fetch(
                        baseUrl + "/api/generate_client",
                        {
                            method: "POST",
                            headers: { "Content-Type": "application/json" },
                            body: JSON.stringify({
                                player_id: currentGeneratingPlayerId,
                                target,
                            }),
                            credentials: "include",
                        },
                    );
                    const contentType =
                        response.headers.get("Content-Type") || "";
                    if (contentType.includes("application/json")) {
                        const data = await response.json();
                        if (data.code === 10086) {
                            showLoginForm();
                            return;
                        }
                        alert(
                            getText("generate_client_failed") +
                                (data.msg || ""),
                        );
                        return;
                    }
                    const blob = await response.blob();
                    const filename =
                        parseDownloadFilename(
                            response.headers.get("Content-Disposition"),
                        ) || `gpipe-client-${currentGeneratingPlayerId}`;
                    const url = URL.createObjectURL(blob);
                    const link = document.createElement("a");
                    link.href = url;
                    link.download = filename;
                    document.body.appendChild(link);
                    link.click();
                    link.remove();
                    URL.revokeObjectURL(url);
                    closeModal("generateClientModal");
                } catch (error) {
                    console.error(error);
                    alert(
                        getText("generate_client_failed") +
                            String(error?.message || error),
                    );
                }
            }

