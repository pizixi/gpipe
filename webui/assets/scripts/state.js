            let currentEditingPlayerId = null;
            let currentEditingTunnelId = null;
            let currentGeneratingPlayerId = null;
            let playerSearchTimer = null;
            let tunnelSearchTimer = null;
            let playerRefreshTimer = null;
            const baseUrl =
                window.location.protocol + "//" + window.location.host;
            let currentLanguage = "zh";
            let playersData = [];
            let tunnelsData = [];
            let clientBuildSettingsData = null;
            let playerSort = { key: "create_time", direction: "desc" };
            let tunnelSort = { key: "receiver", direction: "asc" };
            const playerRefreshIntervalMs = 5000;

            const i18nResources = {
                zh: {
                    login_title: "登录",
                    login_subtitle: "输入后台账号后进入管理系统",
                    login_intro_title: "统一管控玩家、隧道与客户端",
                    login_intro_text:
                        "一个后台完成玩家密钥管理、隧道维护与专属客户端分发。",
                    login_feature_players: "玩家与密钥管理",
                    login_feature_players_desc:
                        "统一维护备注、密钥和在线状态。",
                    login_feature_tunnels: "隧道编排",
                    login_feature_tunnels_desc:
                        "快速查询、修改和启停传输隧道。",
                    login_feature_clients: "客户端生成",
                    login_feature_clients_desc:
                        "为每个玩家生成内置连接参数的专属二进制。",
                    console_title: "管理控制台",
                    console_subtitle: "统一管理玩家、隧道和客户端参数",
                    logout_button: "退出登录",
                    username_placeholder: "用户名",
                    password_placeholder: "密码",
                    remark_placeholder: "备注",
                    login_button: "登录",
                    player_management: "玩家管理",
                    tunnel_management: "隧道管理",
                    client_settings: "客户端设置",
                    client_settings_title: "客户端生成设置",
                    client_settings_subtitle:
                        "设置生成客户端时内置的连接参数和 Shadowsocks 配置。",
                    server_address: "服务端地址",
                    server_address_placeholder:
                        "例如 tcp://127.0.0.1:8118,ws://127.0.0.1:8119",
                    enable_tls: "启用 TLS",
                    tls_server_name: "TLS Server Name",
                    tls_server_name_placeholder: "可选，留空则自动使用主机名",
                    use_shadowsocks: "使用 Shadowsocks",
                    shadowsocks_server: "Shadowsocks 地址",
                    shadowsocks_server_placeholder: "例如 127.0.0.1:8388",
                    shadowsocks_method: "Shadowsocks 方法",
                    shadowsocks_password: "Shadowsocks 密码",
                    shadowsocks_password_placeholder: "请输入 Shadowsocks 密码",
                    save_settings: "保存设置",
                    save_settings_failed: "保存设置失败: ",
                    settings_saved: "设置已保存",
                    generate_client: "生成客户端",
                    client_target: "客户端版本",
                    download_client: "生成并下载",
                    generate_client_failed: "生成客户端失败: ",
                    client_settings_required:
                        "请先在左侧“客户端设置”里填写服务端参数",
                    client_download_hint:
                        "生成后的客户端会自动内置玩家密钥和当前设置，直接运行默认前台模式，同时支持 install / uninstall / run-service 命令。",
                    add_player: "添加玩家",
                    search_player_placeholder: "搜索备注...",
                    search_tunnel_placeholder: "输入玩家ID或描述关键字",
                    search_button: "搜索",
                    id: "ID",
                    player_remark: "备注",
                    player_key: "密钥",
                    create_time: "创建时间",
                    username: "用户名",
                    password: "密码",
                    online_status: "在线状态",
                    actions: "操作",
                    add_tunnel: "添加隧道",
                    source: "监听地址",
                    target: "目标 (IP:端口)",
                    target_tab_title: "目标",
                    type: "类型",
                    protocol_type: "协议类型",
                    sender_id: "隧道出口ID",
                    receiver_id: "隧道入口ID",
                    status: "启用状态",
                    compression_status: "压缩状态",
                    description: "描述",
                    enter_remark_placeholder: "请输入备注",
                    enter_key_placeholder: "请输入密钥",
                    enter_password_placeholder: "请输入密码",
                    cancel_button: "取消",
                    delete_button: "删除",
                    add_button: "添加",
                    edit_player: "修改玩家",
                    enter_new_key_placeholder: "请输入新密钥",
                    generate_key_button: "一键生成",
                    update_button: "更新",
                    enable_compression: "启用压缩",
                    encryption_method: "加密方式",
                    none: "None",
                    aes128: "Aes128",
                    xor: "Xor",
                    description_placeholder: "请输入描述",
                    edit_tunnel: "修改隧道",
                    source_placeholder: "例如 0.0.0.0:1234",
                    target_placeholder: "例如 10.2.20.203:80",
                    sender_id_placeholder: "为0则表示出口端为服务器",
                    receiver_id_placeholder: "为0则表示入口端为服务器",
                    tcp: "TCP",
                    udp: "UDP",
                    socks5: "SOCKS5",
                    http: "HTTP",
                    shadowsocks: "Shadowsocks",
                    online: "在线",
                    offline: "离线",
                    enabled: "启用",
                    disabled: "禁用",
                    compressed: "开启",
                    uncompressed: "关闭",
                    confirm_delete_player: "确定要删除这个玩家吗？",
                    confirm_delete_tunnel: "确定要删除这个隧道吗？",
                    delete_player_failed: "删除玩家失败: ",
                    delete_tunnel_failed: "删除隧道失败: ",
                    add_player_failed: "添加玩家失败: ",
                    update_player_failed: "更新玩家失败: ",
                    online_player_key_locked: "在线玩家不允许修改密钥",
                    online_player_key_locked_hint:
                        "在线玩家仅允许修改备注，密钥需要离线后再改",
                    online_player_delete_forbidden: "在线玩家不允许删除",
                    add_tunnel_failed: "添加隧道失败: ",
                    update_tunnel_failed: "更新隧道失败: ",
                    login_failed: "登录失败: ",
                    shadowsocks_password_required:
                        "Shadowsocks 协议密码不能为空",
                    show_password: "显示密码",
                    hide_password: "隐藏密码",
                    player_not_found: "未找到指定的玩家",
                    tunnel_not_found: "未找到指定的隧道",
                    empty_players: "暂无玩家数据",
                    empty_tunnels: "暂无隧道数据",
                    not_set: "未设置",
                    server_side: "服务器(0)",
                    tunnel_player_picker_placeholder:
                        "搜索玩家ID或备注，留空为服务器(0)",
                    tunnel_player_picker_toggle: "展开玩家列表",
                    tunnel_player_picker_empty: "没有匹配的玩家",
                    tunnel_player_picker_service_hint:
                        "未指定时默认由服务器(0)承担",
                    tunnel_player_picker_manual_hint:
                        "支持直接输入玩家ID",
                    tunnel_player_picker_default_tag: "默认",
                },
                en: {
                    login_title: "Login",
                    login_subtitle: "Sign in to enter the control panel",
                    login_intro_title:
                        "Manage players, tunnels, and clients in one place",
                    login_intro_text:
                        "Use one admin panel for key management, tunnel maintenance, and dedicated client delivery.",
                    login_feature_players: "Player & Key Management",
                    login_feature_players_desc:
                        "Keep remarks, keys, and online status under control.",
                    login_feature_tunnels: "Tunnel Operations",
                    login_feature_tunnels_desc:
                        "Search, edit, and toggle runtime tunnels quickly.",
                    login_feature_clients: "Client Packaging",
                    login_feature_clients_desc:
                        "Generate per-player binaries with embedded connection settings.",
                    console_title: "Control Console",
                    console_subtitle:
                        "Manage players, tunnels, and runtime status in one place",
                    logout_button: "Logout",
                    username_placeholder: "Username",
                    password_placeholder: "Password",
                    remark_placeholder: "Remark",
                    login_button: "Login",
                    player_management: "Players",
                    tunnel_management: "Tunnels",
                    client_settings: "Client Settings",
                    client_settings_title: "Client Build Settings",
                    client_settings_subtitle:
                        "Configure embedded connection and Shadowsocks options for generated clients.",
                    server_address: "Server Address",
                    server_address_placeholder:
                        "For example tcp://127.0.0.1:8118,ws://127.0.0.1:8119",
                    enable_tls: "Enable TLS",
                    tls_server_name: "TLS Server Name",
                    tls_server_name_placeholder:
                        "Optional. Hostname is used when empty.",
                    use_shadowsocks: "Use Shadowsocks",
                    shadowsocks_server: "Shadowsocks Server",
                    shadowsocks_server_placeholder:
                        "For example 127.0.0.1:8388",
                    shadowsocks_method: "Shadowsocks Method",
                    shadowsocks_password: "Shadowsocks Password",
                    shadowsocks_password_placeholder:
                        "Enter the Shadowsocks password",
                    save_settings: "Save Settings",
                    save_settings_failed: "Failed to save settings: ",
                    settings_saved: "Settings saved",
                    generate_client: "Generate Client",
                    client_target: "Client Target",
                    download_client: "Build and Download",
                    generate_client_failed: "Failed to generate client: ",
                    client_settings_required:
                        "Configure the client settings in the left panel first",
                    client_download_hint:
                        "Generated clients embed the player key and current settings, run in foreground by default, and still support install / uninstall / run-service.",
                    add_player: "Add Player",
                    search_player_placeholder: "Search remark...",
                    search_tunnel_placeholder:
                        "Search by player ID or description",
                    search_button: "Search",
                    id: "ID",
                    player_remark: "Remark",
                    player_key: "Key",
                    create_time: "Created",
                    username: "Username",
                    password: "Password",
                    online_status: "Status",
                    actions: "Actions",
                    add_tunnel: "Add Tunnel",
                    source: "Listen Address",
                    target: "Target (IP:Port)",
                    target_tab_title: "Target",
                    type: "Type",
                    protocol_type: "Protocol",
                    sender_id: "Egress ID",
                    receiver_id: "Ingress ID",
                    status: "Status",
                    compression_status: "Compression",
                    description: "Description",
                    enter_remark_placeholder: "Enter remark",
                    enter_key_placeholder: "Enter key",
                    enter_password_placeholder: "Enter password",
                    cancel_button: "Cancel",
                    delete_button: "Delete",
                    add_button: "Add",
                    edit_player: "Edit Player",
                    enter_new_key_placeholder: "Enter new key",
                    generate_key_button: "Generate",
                    update_button: "Update",
                    enable_compression: "Enable Compression",
                    encryption_method: "Encryption",
                    none: "None",
                    aes128: "Aes128",
                    xor: "Xor",
                    description_placeholder: "Enter description",
                    edit_tunnel: "Edit Tunnel",
                    source_placeholder: "0.0.0.0:1234",
                    target_placeholder: "10.2.20.203:80",
                    sender_id_placeholder: "0 = server egress",
                    receiver_id_placeholder: "0 = server ingress",
                    tcp: "TCP",
                    udp: "UDP",
                    socks5: "SOCKS5",
                    http: "HTTP",
                    shadowsocks: "Shadowsocks",
                    online: "Online",
                    offline: "Offline",
                    enabled: "Enabled",
                    disabled: "Disabled",
                    compressed: "On",
                    uncompressed: "Off",
                    confirm_delete_player: "Delete this player?",
                    confirm_delete_tunnel: "Delete this tunnel?",
                    delete_player_failed: "Failed to delete player: ",
                    delete_tunnel_failed: "Failed to delete tunnel: ",
                    add_player_failed: "Failed to add player: ",
                    update_player_failed: "Failed to update player: ",
                    online_player_key_locked:
                        "Online players cannot change the key",
                    online_player_key_locked_hint:
                        "Online players can only edit the remark. Change the key after disconnecting.",
                    online_player_delete_forbidden:
                        "Online players cannot be deleted",
                    add_tunnel_failed: "Failed to add tunnel: ",
                    update_tunnel_failed: "Failed to update tunnel: ",
                    login_failed: "Login failed: ",
                    shadowsocks_password_required:
                        "Shadowsocks password is required",
                    show_password: "Show password",
                    hide_password: "Hide password",
                    player_not_found: "Player not found",
                    tunnel_not_found: "Tunnel not found",
                    empty_players: "No players found",
                    empty_tunnels: "No tunnels found",
                    not_set: "Not set",
                    server_side: "Server (0)",
                    tunnel_player_picker_placeholder:
                        "Search by player ID or remark. Empty = server (0)",
                    tunnel_player_picker_toggle: "Open player list",
                    tunnel_player_picker_empty: "No matching players",
                    tunnel_player_picker_service_hint:
                        "Falls back to server (0) when not specified",
                    tunnel_player_picker_manual_hint:
                        "You can also enter a player ID directly",
                    tunnel_player_picker_default_tag: "Default",
                },
            };

            const defaultShadowsocksMethod = "chacha20-ietf-poly1305";
            const playerKeyCharacters =
                "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789";
            const playerKeyLength = 20;
            const tunnelTransportEncryptionOptions = [
                { value: "None", labelKey: "none" },
                { value: "Aes128", labelKey: "aes128" },
                { value: "Xor", labelKey: "xor" },
            ];
            const tunnelShadowsocksMethodOptions = [
                {
                    value: defaultShadowsocksMethod,
                    label: defaultShadowsocksMethod,
                },
                { value: "aes-128-gcm", label: "aes-128-gcm" },
                { value: "aes-256-gcm", label: "aes-256-gcm" },
            ];
            const clientBuildTargetOptions = [
                {
                    value: "windows-amd64",
                    label: { zh: "Windows x64", en: "Windows x64" },
                },
                {
                    value: "windows-arm64",
                    label: { zh: "Windows ARM64", en: "Windows ARM64" },
                },
                {
                    value: "linux-amd64",
                    label: { zh: "Linux x64", en: "Linux x64" },
                },
                {
                    value: "linux-arm64",
                    label: { zh: "Linux ARM64", en: "Linux ARM64" },
                },
                {
                    value: "linux-armv7",
                    label: {
                        zh: "Linux ARMv7 (armv7l)",
                        en: "Linux ARMv7 (armv7l)",
                    },
                },
            ];

