            function updateLanguage() {
                document.querySelectorAll("[data-i18n]").forEach((element) => {
                    const key = element.getAttribute("data-i18n");
                    if (i18nResources[currentLanguage][key]) {
                        element.textContent =
                            i18nResources[currentLanguage][key];
                    }
                });
                document
                    .querySelectorAll("[data-i18n-placeholder]")
                    .forEach((element) => {
                        const key = element.getAttribute(
                            "data-i18n-placeholder",
                        );
                        if (i18nResources[currentLanguage][key]) {
                            element.setAttribute(
                                "placeholder",
                                i18nResources[currentLanguage][key],
                            );
                        }
                    });
                document
                    .querySelectorAll("select option[data-i18n]")
                    .forEach((option) => {
                        const key = option.getAttribute("data-i18n");
                        if (i18nResources[currentLanguage][key]) {
                            option.textContent =
                                i18nResources[currentLanguage][key];
                        }
                    });
                document.querySelectorAll(".language-btn").forEach((btn) => {
                    if (
                        btn.textContent ===
                        (currentLanguage === "zh" ? "中文" : "EN")
                    ) {
                        btn.classList.add("active");
                    } else {
                        btn.classList.remove("active");
                    }
                });
                updateGenerateKeyButtons();
                renderClientTargetOptions();
                updateEditTunnelPasswordToggle();
                if (typeof refreshTunnelPlayerPickers === "function") {
                    refreshTunnelPlayerPickers();
                }
                toggleTunnelTypeFields("new");
                toggleTunnelTypeFields("edit");
                if (clientBuildSettingsData) {
                    fillClientBuildSettingsForm(clientBuildSettingsData);
                    renderClientSettingsSummary(clientBuildSettingsData);
                }
                if (
                    document.getElementById("mainContent").style.display !==
                    "none"
                ) {
                    loadPlayers();
                    loadTunnels();
                    loadClientBuildSettings();
                }
            }

            function changeLanguage(lang) {
                currentLanguage = lang;
                localStorage.setItem("language", lang);
                updateLanguage();
            }

            async function apiRequest(endpoint, method, data = {}) {
                try {
                    const response = await fetch(baseUrl + endpoint, {
                        method,
                        headers: { "Content-Type": "application/json" },
                        body: JSON.stringify(data),
                        credentials: "include",
                    });
                    return await response.json();
                } catch (error) {
                    console.error("API Error:", error);
                    throw error;
                }
            }

            async function checkLoginStatus() {
                try {
                    const data = await apiRequest("/api/player_list", "POST", {
                        page_number: 0,
                        page_size: 1,
                    });
                    return data.code !== 10086;
                } catch (error) {
                    return false;
                }
            }

            function stopPlayerAutoRefresh() {
                if (playerRefreshTimer !== null) {
                    clearInterval(playerRefreshTimer);
                    playerRefreshTimer = null;
                }
            }

            function startPlayerAutoRefresh() {
                if (playerRefreshTimer !== null) {
                    return;
                }
                playerRefreshTimer = window.setInterval(() => {
                    if (document.hidden) {
                        return;
                    }
                    if (
                        document.getElementById("mainContent").style.display ===
                        "none"
                    ) {
                        return;
                    }
                    loadPlayers();
                }, playerRefreshIntervalMs);
            }

            function showMainContent() {
                document.getElementById("loginForm").style.display = "none";
                document.getElementById("mainContent").style.display = "flex";
                loadPlayers();
                loadTunnels();
                loadClientBuildSettings();
                startPlayerAutoRefresh();
                document.getElementById("defaultOpen").click();
            }

            function showLoginForm() {
                stopPlayerAutoRefresh();
                document.getElementById("loginForm").style.display = "block";
                document.getElementById("mainContent").style.display = "none";
                document.getElementById("password").value = "";
                setTimeout(() => {
                    const input = document.getElementById("username");
                    if (input) {
                        input.focus();
                    }
                }, 0);
            }

            function handleLoginSubmit(event) {
                event.preventDefault();
                login();
            }

            async function login() {
                const username = document
                    .getElementById("username")
                    .value.trim();
                const password = document.getElementById("password").value;
                try {
                    const data = await apiRequest("/api/login", "POST", {
                        username,
                        password,
                    });
                    if (data.code === 0) {
                        showMainContent();
                    } else {
                        alert(getText("login_failed") + data.msg);
                    }
                } catch (error) {}
            }

            async function logout() {
                try {
                    await apiRequest("/api/logout", "POST", {});
                } catch (error) {}
                showLoginForm();
            }


            function handleSearchKeyPress(event, type) {
                if (event.key !== "Enter") {
                    return;
                }
                event.preventDefault();
                if (type === "player") {
                    searchPlayers();
                } else if (type === "tunnel") {
                    renderTunnels();
                }
            }


            function closeModal(modalId) {
                document.getElementById(modalId).style.display = "none";
                if (modalId === "editPlayerModal") {
                    currentEditingPlayerId = null;
                    setEditPlayerKeyLocked(false);
                }
                if (modalId === "generateClientModal") {
                    currentGeneratingPlayerId = null;
                }
                if (modalId === "editTunnelModal") {
                    setEditTunnelPasswordVisibility(false);
                }
                if (
                    (modalId === "addTunnelModal" ||
                        modalId === "editTunnelModal") &&
                    typeof closeAllTunnelPlayerDropdowns === "function"
                ) {
                    closeAllTunnelPlayerDropdowns();
                }
            }


            function openTab(evt, tabName) {
                const tabcontent =
                    document.getElementsByClassName("tabcontent");
                for (let i = 0; i < tabcontent.length; i++) {
                    tabcontent[i].style.display = "none";
                }
                const tablinks = document.getElementsByClassName("tablinks");
                for (let i = 0; i < tablinks.length; i++) {
                    tablinks[i].className = tablinks[i].className.replace(
                        " active",
                        "",
                    );
                }
                const playerToolbar = document.getElementById("playerToolbar");
                const tunnelToolbar = document.getElementById("tunnelToolbar");
                const toolbarActions =
                    document.getElementById("mainToolbarActions");
                if (playerToolbar && tunnelToolbar) {
                    playerToolbar.style.display =
                        tabName === "Players" ? "flex" : "none";
                    tunnelToolbar.style.display =
                        tabName === "Tunnels" ? "flex" : "none";
                }
                if (toolbarActions) {
                    toolbarActions.style.display =
                        tabName === "Settings" ? "none" : "flex";
                }
                document.getElementById(tabName).style.display = "block";
                evt.currentTarget.className += " active";
            }


            window.onload = function () {
                const savedLanguage = localStorage.getItem("language");
                if (savedLanguage) {
                    currentLanguage = savedLanguage;
                }
                updateLanguage();
                checkLoginStatus().then((isLoggedIn) => {
                    if (isLoggedIn) {
                        showMainContent();
                    } else {
                        showLoginForm();
                    }
                });
            };
