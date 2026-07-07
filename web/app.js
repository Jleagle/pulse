// App state
let appStatus = {
    client_configured: false,
    oauth_connected: false,
    user_email: "",
    user_name: "",
    user_picture: ""
};
let currentHorizonDays = 3650; // Query maximum allowed history by default

let appData = {
    overview_loaded: false,
    sleep_sessions: [],
    sleep_next_page_token: "",
    sleep_loaded: false,
    sleep_loading: false,

    rhr_records: [],
    rhr_next_page_token: "",
    hrv_records: [],
    hrv_next_page_token: "",
    heart_loaded: false,
    heart_loading: false,

    activity_records: [],
    activity_next_page_token: "",
    activity_loaded: false,
    activity_loading: false
};

function resetAppData() {
    try { localStorage.removeItem("pulse_app_cache"); } catch (e) {}
    appData = {
        overview_loaded: false,
        sleep_sessions: [],
        sleep_next_page_token: "",
        sleep_loaded: false,
        sleep_loading: false,
        rhr_records: [],
        rhr_next_page_token: "",
        hrv_records: [],
        hrv_next_page_token: "",
        heart_loaded: false,
        heart_loading: false,
        activity_records: [],
        activity_next_page_token: "",
        activity_loaded: false,
        activity_loading: false
    };
    const sleepTbody = document.querySelector("#sleep-history-table tbody");
    if (sleepTbody) sleepTbody.innerHTML = `<tr><td colspan="7" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
    const heartTbody = document.querySelector("#heart-history-table tbody");
    if (heartTbody) heartTbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
    const actTbody = document.querySelector("#activity-history-table tbody");
    if (actTbody) actTbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
    const cards = ["overview-sleep-duration", "overview-rhr", "overview-hrv", "overview-steps"];
    cards.forEach(id => {
        const el = document.getElementById(id);
        if (el) el.textContent = "--";
    });
}

function saveCache() {
    try {
        const cachePayload = {
            horizon: currentHorizonDays,
            timestamp: Date.now(),
            data: {
                overview_loaded: appData.overview_loaded,
                sleep_sessions: appData.sleep_sessions,
                sleep_next_page_token: appData.sleep_next_page_token,
                sleep_loaded: appData.sleep_loaded,
                rhr_records: appData.rhr_records,
                rhr_next_page_token: appData.rhr_next_page_token,
                hrv_records: appData.hrv_records,
                hrv_next_page_token: appData.hrv_next_page_token,
                heart_loaded: appData.heart_loaded,
                activity_records: appData.activity_records,
                activity_next_page_token: appData.activity_next_page_token,
                activity_loaded: appData.activity_loaded
            }
        };
        localStorage.setItem("pulse_app_cache", JSON.stringify(cachePayload));
    } catch (e) {
        console.warn("Failed to save to localStorage:", e);
    }
}

function loadCache() {
    try {
        const raw = localStorage.getItem("pulse_app_cache");
        if (!raw) return false;
        const cached = JSON.parse(raw);
        if (cached.horizon === currentHorizonDays && cached.data && (Date.now() - cached.timestamp < 6 * 60 * 60 * 1000)) {
            Object.assign(appData, cached.data);
            appData.sleep_loading = false;
            appData.heart_loading = false;
            appData.activity_loading = false;
            return true;
        }
    } catch (e) {
        console.warn("Failed to load from localStorage:", e);
    }
    return false;
}

// Global chart references for disposal
const activeCharts = {};

// Handle browser back/forward buttons
window.addEventListener("popstate", () => {
    const pathTab = window.location.pathname.replace("/", "") || "overview";
    const validTab = ["overview", "sleep", "heart", "activity", "settings", "privacy-policy", "terms-of-service"].includes(pathTab) ? pathTab : "overview";
    switchTab(validTab, false);
});

// On page load
document.addEventListener("DOMContentLoaded", () => {
    // Set current date in header if element exists
    const dateEl = document.getElementById("header-date");
    if (dateEl) {
        const options = { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' };
        dateEl.textContent = new Date().toLocaleDateString('en-US', options);
    }

    // Initialize Lucide icons
    lucide.createIcons();

    // Determine target tab from URL path, query param, or hash
    const urlParams = new URLSearchParams(window.location.search);
    const pathTab = window.location.pathname.replace("/", "") || urlParams.get("tab") || window.location.hash.replace("#", "");
    const initialTab = ["overview", "sleep", "heart", "activity", "settings", "privacy-policy", "terms-of-service"].includes(pathTab) ? pathTab : "overview";

    const justConnected = urlParams.get("connected") === "true";
    if (justConnected) {
        showToast("Connected to Google Health successfully!");
        window.history.replaceState({}, document.title, initialTab === "overview" ? "/" : `/${initialTab}`);
    } else if (urlParams.get("error")) {
        showToast("Error connecting: " + decodeURIComponent(urlParams.get("error")), true);
        window.history.replaceState({}, document.title, initialTab === "overview" ? "/" : `/${initialTab}`);
    } else if (initialTab !== "overview") {
        switchTab(initialTab, false);
    }

    // Load setup
    refreshStatus().then(() => {
        if (appStatus.oauth_connected) {
            loadStats();
        } else {
            if (!justConnected) {
                showToast("Previewing sample health metrics in Guest Mode. Click Guest icon or Settings to login!", false);
            }
            loadStats();
        }
    });
});

// Switch UI Tabs
function switchTab(tabId, updateUrl = true) {
    document.querySelectorAll(".tab-pane").forEach(el => el.classList.remove("active"));
    document.querySelectorAll(".nav-item").forEach(el => el.classList.remove("active"));

    const tabEl = document.getElementById("tab-" + tabId);
    if (!tabEl) {
        tabId = "overview";
        document.getElementById("tab-overview").classList.add("active");
        document.getElementById("nav-btn-overview").classList.add("active");
    } else {
        tabEl.classList.add("active");
        const navEl = document.getElementById("nav-btn-" + tabId);
        if (navEl) navEl.classList.add("active");
    }

    if (updateUrl && window.history && window.history.pushState) {
        const newPath = tabId === "overview" ? "/" : `/${tabId}`;
        if (window.location.pathname !== newPath) {
            window.history.pushState({ tab: tabId }, "", newPath);
        }
    }

    const titles = {
        overview: "Dashboard Overview",
        sleep: "Sleep Analysis",
        heart: "Heart Health Trends",
        activity: "Activity & Energy",
        settings: "Application Settings",
        "privacy-policy": "Privacy Policy",
        "terms-of-service": "Terms of Service"
    };
    const titleEl = document.getElementById("page-title");
    if (titleEl) titleEl.textContent = titles[tabId] || "Dashboard";

    if (true) {
        if (tabId === "sleep") {
            if (!appData.sleep_loaded && !appData.sleep_loading) loadMetric("sleep");
            else if (appData.sleep_loading) showLoadingSpinner("sleep");
            else if (appData.sleep_loaded) {
                populateSleepTable(appData.sleep_sessions);
                renderSleepCharts({ sleep_sessions: appData.sleep_sessions });
            }
        } else if (tabId === "heart") {
            if (!appData.heart_loaded && !appData.heart_loading) loadMetric("heart");
            else if (appData.heart_loading) showLoadingSpinner("heart");
            else if (appData.heart_loaded) {
                populateHeartTable(appData.rhr_records, appData.hrv_records);
                renderHeartCharts({ rhr_records: appData.rhr_records, hrv_records: appData.hrv_records });
            }
        } else if (tabId === "activity") {
            if (!appData.activity_loaded && !appData.activity_loading) loadMetric("activity");
            else if (appData.activity_loading) showLoadingSpinner("activity");
            else if (appData.activity_loaded) {
                populateActivityTable(appData.activity_records);
                renderActivityCharts({ activity_records: appData.activity_records });
            }
        } else if (tabId === "overview") {
            if (!appData.overview_loaded && !appData.overview_loading) loadMetric("overview");
            else if (appData.overview_loading) showLoadingSpinner("overview");
            else if (appData.overview_loaded) {
                populateOverviewCards(appData);
                renderOverviewCharts(appData);
            }
        }
    }
}

// Fetch general configuration status
async function refreshStatus() {
    try {
        const res = await fetch("/api/status");
        appStatus = await res.json();

        const authIcon = document.getElementById("auth-status-icon");
        const authTitle = document.getElementById("auth-status-title");
        const authDesc = document.getElementById("auth-status-desc");
        const oauthConfigStatus = document.getElementById("oauth-config-status");
        
        const btnGoogle = document.getElementById("btn-google-connect");
        const btnForceSync = document.getElementById("btn-force-sync");
        const btnPurge = document.getElementById("btn-purge-db");
        
        const welcomeHero = document.getElementById("welcome-hero");
        const userNameEl = document.getElementById("user-name");
        const avatarImg = document.getElementById("avatar-img");
        const avatarBadge = document.getElementById("avatar-badge");

        if (oauthConfigStatus) {
            oauthConfigStatus.textContent = appStatus.client_configured ? "Active & Configured by Host" : "Missing Host Credentials";
            oauthConfigStatus.style.color = appStatus.client_configured ? "var(--color-emerald)" : "var(--color-rose)";
        }

        if (appStatus.oauth_connected) {
            authIcon.className = "success-icon";
            authIcon.setAttribute("data-lucide", "shield-check");
            authTitle.textContent = "Connected to Google Health";
            authDesc.textContent = "Authorized as " + (appStatus.user_email || appStatus.user_name || "Active User");
            
            btnGoogle.disabled = true;
            if (btnForceSync) btnForceSync.disabled = false;
            if (btnPurge) btnPurge.disabled = false;

            if (welcomeHero) welcomeHero.style.display = "none";

            // Update user badge
            userNameEl.textContent = appStatus.user_name || appStatus.user_email || "User";
            if (appStatus.user_picture) {
                avatarImg.src = appStatus.user_picture;
                avatarImg.style.display = "inline-block";
                avatarBadge.style.display = "none";
            } else {
                avatarImg.style.display = "none";
                avatarBadge.style.display = "flex";
                avatarBadge.textContent = (appStatus.user_name || "U")[0].toUpperCase();
            }
        } else {
            authIcon.className = "";
            authIcon.setAttribute("data-lucide", "shield-alert");
            authTitle.textContent = "Not Connected";
            authDesc.textContent = "Link your Google Account to fetch health metrics on-the-fly.";
            
            if (btnForceSync) btnForceSync.disabled = true;
            if (btnPurge) btnPurge.disabled = true;

            if (welcomeHero) welcomeHero.style.display = "block";

            if (btnGoogle) btnGoogle.disabled = false;

            userNameEl.textContent = "Guest";
            avatarImg.style.display = "none";
            avatarBadge.style.display = "flex";
            avatarBadge.textContent = "G";
        }

        lucide.createIcons();
    } catch (e) {
        showToast("Failed to load server status endpoint", true);
    }
}


// Redirect User to Google Consent screen
async function redirectToGoogle() {
    try {
        const res = await fetch("/api/auth-url");
        if (!res.ok) {
            const errData = await res.json().catch(() => ({}));
            throw new Error(errData.error || "Server OAuth credentials (CLIENT_ID / CLIENT_SECRET) not configured.");
        }
        const data = await res.json();
        window.location.href = data.url;
    } catch (e) {
        showToast("Cannot connect: Server OAuth credentials (CLIENT_ID / CLIENT_SECRET environment variables) are not set.", true);
    }
}

// Sign Out & Revoke Session
async function logout() {
    if (!confirm("Are you sure you want to sign out? This will clear your session cookie and empty live memory graphs.")) {
        return;
    }

    try { localStorage.removeItem("pulse_app_cache"); } catch (e) {}

    try {
        const res = await fetch("/api/logout", { method: "POST" });
        if (res.ok) {
            showToast("Signed out successfully.");
            window.location.reload();
        } else {
            showToast("Failed to sign out.", true);
        }
    } catch (e) {
        showToast("Error during logout", true);
    }
}

// Clear Cache and let metrics lazy load when visited
async function triggerRefresh() {
    const btnSettings = document.getElementById("btn-force-sync");
    if (btnSettings) btnSettings.disabled = true;

    try {
        resetAppData();
        showToast("Browser cache cleared! Health data will lazy load when you visit each tab.");
    } catch (e) {
        showToast("Failed to clear cache", true);
    } finally {
        if (btnSettings) btnSettings.disabled = false;
    }
}

// Fetch stats and render UI
async function loadStats(forceRefresh = false) {
    const activeTabPane = document.querySelector(".tab-pane.active");
    const activeTabId = activeTabPane ? activeTabPane.id.replace("tab-", "") : "overview";
    
    if (!forceRefresh && loadCache()) {
        const raw = localStorage.getItem("pulse_app_cache");
        const cacheAgeMin = raw ? Math.round((Date.now() - JSON.parse(raw).timestamp) / 60000) : 0;
        // Suppress green success toast when loading from browser cache as requested by user
        
        if (appData.overview_loaded) {
            populateOverviewCards(appData);
            renderOverviewCharts(appData);
        }
        if (appData.sleep_loaded) {
            populateSleepTable(appData.sleep_sessions);
            renderSleepCharts({ sleep_sessions: appData.sleep_sessions });
        }
        if (appData.heart_loaded) {
            populateHeartTable(appData.rhr_records, appData.hrv_records);
            renderHeartCharts({ rhr_records: appData.rhr_records, hrv_records: appData.hrv_records });
        }
        if (appData.activity_loaded) {
            populateActivityTable(appData.activity_records);
            renderActivityCharts({ activity_records: appData.activity_records });
        }

        if (activeTabId !== "overview" && activeTabId !== "settings" && !appData[`${activeTabId}_loaded`]) {
            await loadMetric(activeTabId);
        }
        return;
    }

    if (activeTabId !== "settings" && activeTabId !== "privacy-policy" && activeTabId !== "terms-of-service") {
        await loadMetric(activeTabId);
    } else {
        await loadMetric("overview");
    }
}

function showLoadingSpinner(metric) {
    const chartMap = {
        overview: ["chart-overview-sleep", "chart-overview-heart"],
        sleep: ["chart-sleep-durations", "chart-sleep-stages"],
        heart: ["chart-hrv", "chart-rhr"],
        rhr: ["chart-hrv", "chart-rhr"],
        hrv: ["chart-hrv", "chart-rhr"],
        activity: ["chart-steps", "chart-calories"]
    };
    const canvases = chartMap[metric] || [];
    canvases.forEach(cid => {
        const canvasEl = document.getElementById(cid);
        if (canvasEl && canvasEl.parentElement) {
            let overlay = canvasEl.parentElement.querySelector(".chart-loading-overlay");
            if (!overlay) {
                overlay = document.createElement("div");
                overlay.className = "chart-loading-overlay";
                canvasEl.parentElement.appendChild(overlay);
            }
            overlay.innerHTML = `<i data-lucide="loader" class="spin" style="width: 28px; height: 28px; color: var(--primary-color);"></i><span>Loading chart data...</span>`;
            overlay.style.display = "flex";
        }
    });

    if (metric === "sleep") {
        const tbody = document.querySelector("#sleep-history-table tbody");
        if (tbody && (!appData.sleep_sessions || appData.sleep_sessions.length === 0)) {
            tbody.innerHTML = `<tr><td colspan="7" style="text-align: center; padding: 3rem 0;"><div style="display: inline-flex; align-items: center; gap: 10px; color: var(--text-muted); font-weight: 500;"><i data-lucide="loader" class="spin" style="width: 22px; height: 22px; color: var(--primary-color);"></i> Loading sleep history from Google Health...</div></td></tr>`;
        }
    } else if (metric === "heart" || metric === "rhr" || metric === "hrv") {
        const tbody = document.querySelector("#heart-history-table tbody");
        if (tbody && (!appData.rhr_records || appData.rhr_records.length === 0)) {
            tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; padding: 3rem 0;"><div style="display: inline-flex; align-items: center; gap: 10px; color: var(--text-muted); font-weight: 500;"><i data-lucide="loader" class="spin" style="width: 22px; height: 22px; color: var(--primary-color);"></i> Loading heart vitals from Google Health...</div></td></tr>`;
        }
    } else if (metric === "activity") {
        const tbody = document.querySelector("#activity-history-table tbody");
        if (tbody && (!appData.activity_records || appData.activity_records.length === 0)) {
            tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; padding: 3rem 0;"><div style="display: inline-flex; align-items: center; gap: 10px; color: var(--text-muted); font-weight: 500;"><i data-lucide="loader" class="spin" style="width: 22px; height: 22px; color: var(--primary-color);"></i> Loading activity log from Google Health...</div></td></tr>`;
        }
    } else if (metric === "overview") {
        const cards = ["overview-sleep-duration", "overview-rhr", "overview-hrv", "overview-steps"];
        cards.forEach(id => {
            const el = document.getElementById(id);
            if (el && (el.textContent.includes("--") || el.textContent.trim() === "")) {
                el.innerHTML = `<i data-lucide="loader" class="spin" style="width: 22px; height: 22px; display: inline-block; color: var(--primary-color);"></i>`;
            }
        });
    }
    if (typeof lucide !== "undefined" && lucide.createIcons) lucide.createIcons();
}

function hideLoadingSpinner(metric) {
    const chartMap = {
        overview: ["chart-overview-sleep", "chart-overview-heart"],
        sleep: ["chart-sleep-durations", "chart-sleep-stages"],
        heart: ["chart-hrv", "chart-rhr"],
        rhr: ["chart-hrv", "chart-rhr"],
        hrv: ["chart-hrv", "chart-rhr"],
        activity: ["chart-steps", "chart-calories"]
    };
    const canvases = chartMap[metric] || [];
    canvases.forEach(cid => {
        const canvasEl = document.getElementById(cid);
        if (canvasEl && canvasEl.parentElement) {
            const overlay = canvasEl.parentElement.querySelector(".chart-loading-overlay");
            if (overlay) overlay.remove();
        }
    });
}

async function loadMetric(metric, pageToken = "") {
    if (appData[`${metric}_loading`]) return;
    appData[`${metric}_loading`] = true;
    showLoadingSpinner(metric);

    try {
        let url = `/api/stats?limit=${currentHorizonDays}&metric=${metric}`;
        if (pageToken) {
            url += `&pageToken=${encodeURIComponent(pageToken)}`;
        }
        const res = await fetch(url);
        if (!res.ok) {
            if (res.status === 401) {
                appStatus.oauth_connected = false;
                await refreshStatus();
            } else if (res.status === 429) {
                const errJson = await res.json().catch(() => ({}));
                const retrySec = errJson.retry_after || 60;
                showToast(`Rate limited by Google API. Paused for ${retrySec} seconds...`, true);
            }
            return;
        }

        const data = await res.json();

        if (metric === "overview") {
            appData.overview_loaded = true;
            if (!appData.sleep_loaded) appData.sleep_sessions = data.sleep_sessions || [];
            if (!appData.heart_loaded) {
                appData.rhr_records = data.rhr_records || [];
                appData.hrv_records = data.hrv_records || [];
            }
            if (!appData.activity_loaded) appData.activity_records = data.activity_records || [];
            populateOverviewCards(appData);
            renderOverviewCharts(appData);
        } else if (metric === "sleep") {
            if (pageToken) {
                appData.sleep_sessions = appData.sleep_sessions.concat(data.sleep_sessions || []);
            } else {
                appData.sleep_sessions = data.sleep_sessions || [];
            }
            appData.sleep_next_page_token = data.sleep_next_page_token || "";
            appData.sleep_loaded = true;
            populateSleepTable(appData.sleep_sessions);
            renderSleepCharts({ sleep_sessions: appData.sleep_sessions });
        } else if (metric === "heart" || metric === "rhr" || metric === "hrv") {
            if (metric === "heart" || metric === "rhr") {
                if (pageToken && metric === "rhr") {
                    appData.rhr_records = appData.rhr_records.concat(data.rhr_records || []);
                } else {
                    appData.rhr_records = data.rhr_records || [];
                }
                appData.rhr_next_page_token = data.rhr_next_page_token || "";
            }
            if (metric === "heart" || metric === "hrv") {
                if (pageToken && metric === "hrv") {
                    appData.hrv_records = appData.hrv_records.concat(data.hrv_records || []);
                } else {
                    appData.hrv_records = data.hrv_records || [];
                }
                appData.hrv_next_page_token = data.hrv_next_page_token || "";
            }
            populateHeartTable(appData.rhr_records, appData.hrv_records);
            appData.heart_loaded = true;
            renderHeartCharts({ rhr_records: appData.rhr_records, hrv_records: appData.hrv_records });
        } else if (metric === "activity") {
            if (pageToken) {
                appData.activity_records = appData.activity_records.concat(data.activity_records || []);
            } else {
                appData.activity_records = data.activity_records || [];
            }
            appData.activity_next_page_token = data.activity_next_page_token || "";
            appData.activity_loaded = true;
            populateActivityTable(appData.activity_records);
            renderActivityCharts({ activity_records: appData.activity_records });
        }
        saveCache();
    } catch (e) {
        showToast(`Failed to load ${metric} data`, true);
    } finally {
        appData[`${metric}_loading`] = false;
        hideLoadingSpinner(metric);
        if (!appData[`${metric}_loaded`]) {
            if (metric === "sleep") populateSleepTable(appData.sleep_sessions || []);
            else if (metric === "heart") populateHeartTable(appData.rhr_records || [], appData.hrv_records || []);
            else if (metric === "activity") populateActivityTable(appData.activity_records || []);
            else if (metric === "overview") populateOverviewCards(appData);
        }
    }
}

function populateOverviewCards(data) {
    // Sleep Card
    const sleepVal = document.getElementById("overview-sleep-duration");
    const sleepSub = document.getElementById("overview-sleep-type");
    if (data && data.sleep_sessions && data.sleep_sessions.length > 0) {
        const lastSleep = data.sleep_sessions[0];
        const hrs = Math.floor(lastSleep.duration_minutes / 60);
        const mins = lastSleep.duration_minutes % 60;
        if (sleepVal) sleepVal.textContent = `${hrs}h ${mins}m`;
        
        const sleepDate = new Date(lastSleep.start_time).toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
        if (sleepSub) sleepSub.textContent = `Asleep ${lastSleep.minutes_asleep}m (${lastSleep.sleep_type} type) on ${sleepDate}`;
    } else {
        if (sleepVal) sleepVal.textContent = "--h --m";
        if (sleepSub) sleepSub.textContent = "No sleep data recorded";
    }

    // RHR Card
    const rhrVal = document.getElementById("overview-rhr");
    const rhrSub = document.getElementById("overview-rhr-change");
    if (data && data.rhr_records && data.rhr_records.length > 0) {
        const latestRHR = data.rhr_records[0];
        if (rhrVal) rhrVal.innerHTML = `${latestRHR.beats_per_minute} <span class="unit">BPM</span>`;
        if (rhrSub) rhrSub.textContent = `Recorded on ${latestRHR.date}`;
    } else {
        if (rhrVal) rhrVal.innerHTML = `-- <span class="unit">BPM</span>`;
        if (rhrSub) rhrSub.textContent = "No heart rate data";
    }

    // HRV Card
    const hrvVal = document.getElementById("overview-hrv");
    const hrvSub = document.getElementById("overview-hrv-details");
    if (data && data.hrv_records && data.hrv_records.length > 0) {
        const latestHRV = data.hrv_records[0];
        if (hrvVal) hrvVal.innerHTML = `${Math.round(latestHRV.avg_hrv_ms)} <span class="unit">ms</span>`;
        
        let details = `Date: ${latestHRV.date}`;
        if (latestHRV.deep_sleep_rmssd) {
            details += ` | Deep Sleep RMSSD: ${Math.round(latestHRV.deep_sleep_rmssd)}ms`;
        }
        if (hrvSub) hrvSub.textContent = details;
    } else {
        if (hrvVal) hrvVal.innerHTML = `-- <span class="unit">ms</span>`;
        if (hrvSub) hrvSub.textContent = "No HRV data";
    }

    // Steps Card
    const stepsVal = document.getElementById("overview-steps");
    const calVal = document.getElementById("overview-calories");
    if (data && data.activity_records && data.activity_records.length > 0) {
        const latestAct = data.activity_records[0];
        if (stepsVal) stepsVal.textContent = latestAct.steps.toLocaleString();
        if (calVal) calVal.textContent = `${latestAct.calories_burned} kcal | ${latestAct.active_minutes} active mins`;
    } else {
        if (stepsVal) stepsVal.textContent = "--,---";
        if (calVal) calVal.textContent = "-- kcal burned";
    }
}

function populateSleepTable(sessions) {
    const sleepTbody = document.querySelector("#sleep-history-table tbody");
    if (!sleepTbody) return;
    sleepTbody.innerHTML = "";
    if (sessions && sessions.length > 0) {
        sessions.forEach(s => {
            const tr = document.createElement("tr");
            const st = new Date(s.start_time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
            const et = new Date(s.end_time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
            const sDate = new Date(s.start_time).toLocaleDateString([], { month: 'short', day: 'numeric', year: 'numeric' });
            
            const hrs = Math.floor(s.duration_minutes / 60);
            const mins = s.duration_minutes % 60;
            
            tr.innerHTML = `
                <td>${sDate}</td>
                <td>${st}</td>
                <td>${et}</td>
                <td>${hrs}h ${mins}m</td>
                <td>${s.minutes_asleep}m</td>
                <td>${s.minutes_awake}m</td>
                <td><span class="badge badge-outline">${s.sleep_type}</span></td>
            `;
            sleepTbody.appendChild(tr);
        });
    } else {
        sleepTbody.innerHTML = `<tr><td colspan="7" style="text-align: center; color: var(--text-muted);">No records found. Click sync to retrieve your health statistics.</td></tr>`;
    }
}

function populateHeartTable(rhrRecords, hrvRecords) {
    const tbody = document.querySelector("#heart-history-table tbody");
    if (!tbody) return;
    tbody.innerHTML = "";

    const rhrMap = {};
    (rhrRecords || []).forEach(r => rhrMap[r.date] = r);
    const hrvMap = {};
    (hrvRecords || []).forEach(r => hrvMap[r.date] = r);

    const dates = Array.from(new Set([
        ...Object.keys(rhrMap),
        ...Object.keys(hrvMap)
    ])).sort().reverse();

    if (dates.length > 0) {
        dates.forEach(d => {
            const rhr = rhrMap[d];
            const hrv = hrvMap[d];
            const tr = document.createElement("tr");
            const rhrVal = rhr ? `<strong>${rhr.beats_per_minute}</strong> BPM` : "--";
            const hrvVal = hrv ? `<strong>${Math.round(hrv.avg_hrv_ms)}</strong> ms` : "--";
            const deepVal = (hrv && hrv.deep_sleep_rmssd) ? `${Math.round(hrv.deep_sleep_rmssd)} ms` : "--";
            
            let status = `<span class="badge badge-normal">Normal</span>`;
            if (rhr && rhr.beats_per_minute > 80) status = `<span class="badge badge-warning">Elevated RHR</span>`;
            else if (hrv && hrv.avg_hrv_ms > 60) status = `<span class="badge badge-optimal">Optimal HRV</span>`;

            tr.innerHTML = `
                <td>${d}</td>
                <td>${rhrVal}</td>
                <td>${hrvVal}</td>
                <td>${deepVal}</td>
                <td>${status}</td>
            `;
            tbody.appendChild(tr);
        });
    } else {
        tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
    }
}

function populateActivityTable(records) {
    const actTbody = document.querySelector("#activity-history-table tbody");
    if (!actTbody) return;
    actTbody.innerHTML = "";
    if (records && records.length > 0) {
        records.forEach(a => {
            const tr = document.createElement("tr");
            const goalMet = a.steps >= 10000;
            const badgeClass = goalMet ? "badge badge-success" : "badge badge-outline";
            const iconName = goalMet ? "check-circle-2" : "circle-dashed";
            const goalText = goalMet ? "Yes" : "No";
            tr.innerHTML = `
                <td>${a.date}</td>
                <td><strong>${a.steps.toLocaleString()}</strong></td>
                <td>${a.calories_burned} kcal</td>
                <td>${a.active_minutes} mins</td>
                <td><span class="${badgeClass}" style="gap: 6px;"><i data-lucide="${iconName}" style="width: 15px; height: 15px;"></i>${goalText} (${Math.round((a.steps / 10000) * 100)}%)</span></td>
            `;
            actTbody.appendChild(tr);
        });
        if (typeof lucide !== "undefined" && lucide.createIcons) lucide.createIcons();
    } else {
        actTbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
    }
}

function checkInfiniteScroll() {
    const activeTabPane = document.querySelector(".tab-pane.active");
    if (!activeTabPane) return;

    const tabId = activeTabPane.id.replace("tab-", "");
    
    const windowScrolledNearBottom = (window.innerHeight + window.scrollY) >= (document.documentElement.scrollHeight - 300);

    let tableScrolledNearBottom = false;
    const tableContainers = activeTabPane.querySelectorAll(".table-responsive, .table-container");
    tableContainers.forEach(container => {
        if (container.scrollHeight > container.clientHeight && (container.scrollTop + container.clientHeight) >= (container.scrollHeight - 150)) {
            tableScrolledNearBottom = true;
        }
    });

    if (windowScrolledNearBottom || tableScrolledNearBottom) {
        if (tabId === "sleep" && appData.sleep_next_page_token && !appData.sleep_loading) {
            showToast("Loading more sleep history...");
            loadMetric("sleep", appData.sleep_next_page_token);
        } else if (tabId === "heart") {
            if (appData.rhr_next_page_token && !appData.heart_loading) {
                showToast("Loading more heart rate history...");
                loadMetric("rhr", appData.rhr_next_page_token);
            } else if (appData.hrv_next_page_token && !appData.heart_loading) {
                showToast("Loading more HRV history...");
                loadMetric("hrv", appData.hrv_next_page_token);
            }
        } else if (tabId === "activity" && appData.activity_next_page_token && !appData.activity_loading) {
            showToast("Loading more activity history...");
            loadMetric("activity", appData.activity_next_page_token);
        }
    }
}

window.addEventListener("scroll", checkInfiniteScroll, { passive: true });
document.addEventListener("scroll", function(e) {
    if (e.target && e.target.classList && (e.target.classList.contains("table-responsive") || e.target.classList.contains("table-container"))) {
        checkInfiniteScroll();
    }
}, { passive: true, capture: true });

// Chart rendering helpers
function renderChart(canvasId, config) {
    const el = document.getElementById(canvasId);
    if (!el) {
        console.warn(`Canvas element with ID '${canvasId}' not found.`);
        return;
    }
    if (el.parentElement) {
        const overlay = el.parentElement.querySelector(".chart-loading-overlay");
        if (overlay) overlay.remove();
    }
    if (activeCharts[canvasId]) {
        activeCharts[canvasId].destroy();
    }
    const ctx = el.getContext("2d");
    activeCharts[canvasId] = new Chart(ctx, config);
}

function renderOverviewCharts(data) {
    // 1. Sleep Duration Trend on Overview
    if (data.sleep_sessions && data.sleep_sessions.length > 0) {
        const recentSleeps = [...data.sleep_sessions].slice(0, 14).reverse();
        const dates = recentSleeps.map(s => new Date(s.start_time).toLocaleDateString([], { month: 'short', day: 'numeric' }));
        renderChart("chart-overview-sleep", {
            type: 'line',
            data: {
                labels: dates,
                datasets: [
                    {
                        label: 'Minutes Asleep',
                        data: recentSleeps.map(s => s.minutes_asleep),
                        borderColor: '#c084fc',
                        backgroundColor: 'rgba(192, 132, 252, 0.05)',
                        tension: 0.35,
                        fill: true,
                        borderWidth: 3,
                    },
                    {
                        label: 'Minutes Awake',
                        data: recentSleeps.map(s => s.minutes_awake),
                        borderColor: '#f43f5e',
                        backgroundColor: 'rgba(244, 63, 94, 0.05)',
                        tension: 0.35,
                        fill: true,
                        borderWidth: 2,
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { labels: { color: '#94a3b8' } } },
                scales: {
                    x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                    y: { min: 0, beginAtZero: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'Minutes', color: '#94a3b8' } }
                }
            }
        });
    }

    // 2. Heart Recovery Trend on Overview
    if ((data.rhr_records && data.rhr_records.length > 0) || (data.hrv_records && data.hrv_records.length > 0)) {
        const rhrMap = {};
        (data.rhr_records || []).forEach(r => rhrMap[r.date] = r.beats_per_minute);
        const hrvMap = {};
        (data.hrv_records || []).forEach(r => hrvMap[r.date] = r.avg_hrv_ms);

        const dates = Array.from(new Set([
            ...(data.rhr_records || []).map(r => r.date),
            ...(data.hrv_records || []).map(r => r.date)
        ])).sort().slice(-14);

        const labels = dates.map(d => new Date(d).toLocaleDateString([], { month: 'short', day: 'numeric' }));
        const rhrPoints = dates.map(d => rhrMap[d] || null);
        const hrvPoints = dates.map(d => hrvMap[d] || null);

        renderChart("chart-overview-heart", {
            type: 'line',
            data: {
                labels: labels,
                datasets: [
                    {
                        label: 'HRV (ms)',
                        data: hrvPoints,
                        borderColor: '#38bdf8',
                        pointBackgroundColor: '#38bdf8',
                        borderWidth: 3,
                        tension: 0.3,
                        yAxisID: 'y'
                    },
                    {
                        label: 'RHR (BPM)',
                        data: rhrPoints,
                        borderColor: '#f43f5e',
                        pointBackgroundColor: '#f43f5e',
                        borderWidth: 3,
                        tension: 0.3,
                        yAxisID: 'y1'
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { labels: { color: '#94a3b8' } } },
                scales: {
                    x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                    y: {
                        type: 'linear', position: 'left', min: 0, beginAtZero: true,
                        grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' },
                        title: { display: true, text: 'HRV (ms)', color: '#38bdf8' }
                    },
                    y1: {
                        type: 'linear', position: 'right',
                        grid: { drawOnChartArea: false }, ticks: { color: '#94a3b8' },
                        title: { display: true, text: 'RHR (BPM)', color: '#f43f5e' }
                    }
                }
            }
        });
    }
}

function renderSleepCharts(data) {
    if (!data.sleep_sessions || data.sleep_sessions.length === 0) return;

    // Limit to last 14 nights, ordered oldest to newest for chronological trend
    const recentSleeps = [...data.sleep_sessions].slice(0, 14).reverse();
    const dates = recentSleeps.map(s => new Date(s.start_time).toLocaleDateString([], { month: 'short', day: 'numeric' }));
    
    // Chart 1: Sleep Durations Trend (Asleep vs Awake)
    renderChart("chart-sleep-durations", {
        type: 'line',
        data: {
            labels: dates,
            datasets: [
                {
                    label: 'Minutes Asleep',
                    data: recentSleeps.map(s => s.minutes_asleep),
                    borderColor: '#c084fc',
                    backgroundColor: 'rgba(192, 132, 252, 0.05)',
                    tension: 0.35,
                    fill: true,
                    borderWidth: 3,
                },
                {
                    label: 'Minutes Awake',
                    data: recentSleeps.map(s => s.minutes_awake),
                    borderColor: '#f43f5e',
                    backgroundColor: 'rgba(244, 63, 94, 0.05)',
                    tension: 0.35,
                    fill: true,
                    borderWidth: 2,
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: { legend: { labels: { color: '#94a3b8' } } },
            scales: {
                x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                y: { min: 0, beginAtZero: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'Minutes', color: '#94a3b8' } }
            }
        }
    });

    // Chart 2: Stacked sleep stages
    const deepData = [];
    const remData = [];
    const lightData = [];
    const awakeData = [];

    recentSleeps.forEach(s => {
        let rem = 0, deep = 0, light = 0, awake = 0;
        if (s.stages && s.stages.length > 0) {
            s.stages.forEach(st => {
                const dur = st.duration_minutes;
                switch (st.stage_type) {
                    case "REM": rem += dur; break;
                    case "DEEP": deep += dur; break;
                    case "LIGHT": case "ASLEEP": light += dur; break;
                    case "AWAKE": case "RESTLESS": awake += dur; break;
                }
            });
        } else {
            rem = Math.round(s.minutes_asleep * 0.20);
            deep = Math.round(s.minutes_asleep * 0.15);
            light = s.minutes_asleep - rem - deep;
            awake = s.minutes_awake;
        }
        deepData.push(deep);
        remData.push(rem);
        lightData.push(light);
        awakeData.push(awake);
    });

    renderChart("chart-sleep-stages", {
        type: 'bar',
        data: {
            labels: dates,
            datasets: [
                { label: 'Deep Sleep', data: deepData, backgroundColor: '#a855f7' },
                { label: 'REM Sleep', data: remData, backgroundColor: '#38bdf8' },
                { label: 'Light Sleep', data: lightData, backgroundColor: '#818cf8' },
                { label: 'Awake', data: awakeData, backgroundColor: '#f43f5e' }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: { legend: { labels: { color: '#94a3b8' } } },
            scales: {
                x: { stacked: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                y: { stacked: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'Total Minutes', color: '#94a3b8' } }
            }
        }
    });
}

function renderHeartCharts(data) {
    if ((!data.rhr_records || data.rhr_records.length === 0) && (!data.hrv_records || data.hrv_records.length === 0)) return;

    // 1. HRV Chart
    if (data.hrv_records && data.hrv_records.length > 0) {
        const hrvDates = [...data.hrv_records].slice(0, 14).reverse();
        renderChart("chart-hrv", {
            type: 'line',
            data: {
                labels: hrvDates.map(r => new Date(r.date).toLocaleDateString([], { month: 'short', day: 'numeric' })),
                datasets: [{
                    label: 'HRV (ms)',
                    data: hrvDates.map(r => Math.round(r.avg_hrv_ms)),
                    borderColor: '#38bdf8',
                    backgroundColor: 'rgba(56, 189, 248, 0.05)',
                    borderWidth: 3,
                    pointBackgroundColor: '#38bdf8',
                    tension: 0.3,
                    fill: true
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: {
                    x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                    y: { min: 0, beginAtZero: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'ms', color: '#38bdf8' } }
                }
            }
        });
    }

    // 2. RHR Chart
    if (data.rhr_records && data.rhr_records.length > 0) {
        const rhrDates = [...data.rhr_records].slice(0, 14).reverse();
        renderChart("chart-rhr", {
            type: 'line',
            data: {
                labels: rhrDates.map(r => new Date(r.date).toLocaleDateString([], { month: 'short', day: 'numeric' })),
                datasets: [{
                    label: 'Resting Heart Rate (BPM)',
                    data: rhrDates.map(r => r.beats_per_minute),
                    borderColor: '#f43f5e',
                    backgroundColor: 'rgba(244, 63, 94, 0.05)',
                    borderWidth: 3,
                    pointBackgroundColor: '#f43f5e',
                    tension: 0.3,
                    fill: true
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: {
                    x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                    y: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'BPM', color: '#f43f5e' } }
                }
            }
        });
    }
}

function renderActivityCharts(data) {
    if (!data.activity_records || data.activity_records.length === 0) return;

    const recentAct = [...data.activity_records].slice(0, 14).reverse();
    const dates = recentAct.map(a => new Date(a.date).toLocaleDateString([], { month: 'short', day: 'numeric' }));

    // Chart 1: Steps Trend
    renderChart("chart-steps", {
        type: 'bar',
        data: {
            labels: dates,
            datasets: [{
                label: 'Daily Steps',
                data: recentAct.map(a => a.steps),
                backgroundColor: 'rgba(52, 211, 153, 0.5)',
                borderColor: '#34d399',
                borderWidth: 2,
                borderRadius: 4
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: { legend: { labels: { color: '#94a3b8' } } },
            scales: {
                x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                y: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } }
            }
        }
    });

    // Chart 2: Calories burned
    renderChart("chart-calories", {
        type: 'bar',
        data: {
            labels: dates,
            datasets: [{
                label: 'Active Energy (kcal)',
                data: recentAct.map(a => a.calories_burned),
                backgroundColor: 'rgba(244, 63, 94, 0.5)',
                borderColor: '#f43f5e',
                borderWidth: 2,
                borderRadius: 4
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: { legend: { labels: { color: '#94a3b8' } } },
            scales: {
                x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                y: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } }
            }
        }
    });
}

// Toast Alert display
function showToast(message, isError = false) {
    // Create element
    const toast = document.createElement("div");
    toast.className = `toast-banner ${isError ? 'toast-error' : 'toast-success'}`;
    toast.innerHTML = `
        <div class="toast-content">
            <i data-lucide="${isError ? 'alert-triangle' : 'info'}"></i>
            <span>${message}</span>
        </div>
    `;
    
    // Add toast stylesheet parameters dynamically
    if (!document.getElementById("toast-styles")) {
        const styles = document.createElement("style");
        styles.id = "toast-styles";
        styles.innerHTML = `
            .toast-banner {
                position: fixed;
                bottom: 24px;
                right: 24px;
                padding: 16px 24px;
                border-radius: 12px;
                color: #ffffff;
                font-weight: 500;
                font-size: 0.95rem;
                box-shadow: 0 10px 30px rgba(0,0,0,0.5);
                backdrop-filter: blur(12px);
                z-index: 9999;
                animation: toastSlideIn 0.3s cubic-bezier(0.4, 0, 0.2, 1) forwards;
                border: 1px solid rgba(255,255,255,0.1);
            }
            .toast-success {
                background: rgba(16, 185, 129, 0.9);
                box-shadow: 0 10px 30px rgba(16, 185, 129, 0.25);
            }
            .toast-error {
                background: rgba(239, 68, 68, 0.9);
                box-shadow: 0 10px 30px rgba(239, 68, 68, 0.25);
            }
            .toast-content {
                display: flex;
                align-items: center;
                gap: 12px;
            }
            .toast-content i {
                width: 20px;
                height: 20px;
            }
            @keyframes toastSlideIn {
                from { transform: translateY(40px) scale(0.95); opacity: 0; }
                to { transform: translateY(0) scale(1); opacity: 1; }
            }
            @keyframes toastSlideOut {
                from { transform: translateY(0) scale(1); opacity: 1; }
                to { transform: translateY(30px) scale(0.95); opacity: 0; }
            }
        `;
        document.head.appendChild(styles);
    }

    document.body.appendChild(toast);
    lucide.createIcons();

    // Remove after 4 seconds
    setTimeout(() => {
        toast.style.animation = "toastSlideOut 0.3s cubic-bezier(0.4, 0, 0.2, 1) forwards";
        setTimeout(() => toast.remove(), 300);
    }, 4000);
}
