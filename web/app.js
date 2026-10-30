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
    hr_zone_records: [],
    vo2_max_records: [],
    heart_loaded: false,
    heart_loading: false,

    activity_records: [],
    activity_next_page_token: "",
    activity_loaded: false,
    activity_loading: false,

    exercise_sessions: [],
    exercise_next_page_token: "",
    workouts_loaded: false,
    workouts_loading: false,

    spo2_records: [],
    respiratory_records: [],
    sleep_temp_records: [],
    body_records: [],
    vitals_loaded: false,
    vitals_loading: false,

    nutrition_records: [],
    nutrition_loaded: false,
    nutrition_loading: false
};

let workoutMap = null;
let workoutMapLoaded = false;
let pendingRouteRender = null;
let workoutMarkers = [];
let selectedExerciseId = "";
let selectedSleepId = "";

const CACHE_KEY = "pulse_app_cache_v2";

function resetAppData() {
    try {
        localStorage.removeItem("pulse_app_cache");
        localStorage.removeItem(CACHE_KEY);
    } catch (e) {}
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
        hr_zone_records: [],
        vo2_max_records: [],
        heart_loaded: false,
        heart_loading: false,
        activity_records: [],
        activity_next_page_token: "",
        activity_loaded: false,
        activity_loading: false,
        exercise_sessions: [],
        exercise_next_page_token: "",
        workouts_loaded: false,
        workouts_loading: false,
        spo2_records: [],
        respiratory_records: [],
        sleep_temp_records: [],
        body_records: [],
        vitals_loaded: false,
        vitals_loading: false,
        nutrition_records: [],
        nutrition_loaded: false,
        nutrition_loading: false
    };

    // Destroy any active charts
    Object.keys(activeCharts).forEach(cid => {
        if (activeCharts[cid]) {
            activeCharts[cid].destroy();
            delete activeCharts[cid];
        }
    });

    // Clear workout map route & markers
    workoutMarkers.forEach(m => m.remove());
    workoutMarkers = [];
    pendingRouteRender = null;
    if (workoutMap && workoutMapLoaded) {
        const routeSource = workoutMap.getSource("workout-route");
        if (routeSource) {
            routeSource.setData({ type: "FeatureCollection", features: [] });
        }
    }

    const sleepTbody = document.querySelector("#sleep-history-table tbody");
    if (sleepTbody) sleepTbody.innerHTML = `<tr><td colspan="8" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
    const heartTbody = document.querySelector("#heart-history-table tbody");
    if (heartTbody) heartTbody.innerHTML = `<tr><td colspan="7" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
    const actTbody = document.querySelector("#activity-history-table tbody");
    if (actTbody) actTbody.innerHTML = `<tr><td colspan="7" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
    const workTbody = document.querySelector("#workouts-history-table tbody");
    if (workTbody) workTbody.innerHTML = `<tr><td colspan="9" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
    const vitalsTbody = document.querySelector("#vitals-history-table tbody");
    if (vitalsTbody) vitalsTbody.innerHTML = `<tr><td colspan="6" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
    const nutTbody = document.querySelector("#nutrition-history-table tbody");
    if (nutTbody) nutTbody.innerHTML = `<tr><td colspan="7" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
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
                hr_zone_records: appData.hr_zone_records,
                vo2_max_records: appData.vo2_max_records,
                heart_loaded: appData.heart_loaded,
                activity_records: appData.activity_records,
                activity_next_page_token: appData.activity_next_page_token,
                activity_loaded: appData.activity_loaded,
                exercise_sessions: appData.exercise_sessions,
                exercise_next_page_token: appData.exercise_next_page_token,
                workouts_loaded: appData.workouts_loaded,
                spo2_records: appData.spo2_records,
                respiratory_records: appData.respiratory_records,
                sleep_temp_records: appData.sleep_temp_records,
                body_records: appData.body_records,
                vitals_loaded: appData.vitals_loaded,
                nutrition_records: appData.nutrition_records,
                nutrition_loaded: appData.nutrition_loaded
            }
        };
        localStorage.setItem(CACHE_KEY, JSON.stringify(cachePayload));
    } catch (e) {
        console.warn("Failed to save to localStorage:", e);
    }
}

function loadCache() {
    try {
        const raw = localStorage.getItem(CACHE_KEY);
        if (!raw) return false;
        const cached = JSON.parse(raw);
        if (cached.horizon === currentHorizonDays && cached.data && (Date.now() - cached.timestamp < 6 * 60 * 60 * 1000)) {
            Object.assign(appData, cached.data);
            appData.sleep_loading = false;
            appData.heart_loading = false;
            appData.activity_loading = false;
            appData.workouts_loading = false;
            appData.vitals_loading = false;
            appData.nutrition_loading = false;
            return true;
        }
    } catch (e) {
        console.warn("Failed to load from localStorage:", e);
    }
    return false;
}

// Global chart references for disposal
const activeCharts = {};

const VALID_TABS = ["overview", "workouts", "sleep", "heart", "activity", "vitals", "nutrition", "settings"];

// Handle browser back/forward buttons
window.addEventListener("popstate", () => {
    const pathTab = window.location.pathname.replace("/", "") || "overview";
    const validTab = VALID_TABS.includes(pathTab) ? pathTab : "overview";
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
    const initialTab = VALID_TABS.includes(pathTab) ? pathTab : "overview";

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
        loadStats();
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
        workouts: "Workouts & GPS Route Maps",
        sleep: "Sleep Analysis",
        heart: "Heart & Cardio Fitness",
        activity: "Activity & Energy",
        vitals: "Vitals & Body Composition",
        nutrition: "Nutrition & Hydration",
        settings: "Application Settings"
    };
    const titleEl = document.getElementById("page-title");
    if (titleEl) titleEl.textContent = titles[tabId] || "Dashboard";

    if (tabId === "workouts") {
        setTimeout(() => {
            initWorkoutMap();
            if (workoutMap) workoutMap.resize();
        }, 50);
    }

    if (!appStatus.oauth_connected) {
        return;
    }

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
            populateHeartTable(appData.rhr_records, appData.hrv_records, appData.vo2_max_records, appData.hr_zone_records);
            renderHeartCharts(appData);
        }
    } else if (tabId === "activity") {
        if (!appData.activity_loaded && !appData.activity_loading) loadMetric("activity");
        else if (appData.activity_loading) showLoadingSpinner("activity");
        else if (appData.activity_loaded) {
            populateActivityTable(appData.activity_records);
            renderActivityCharts({ activity_records: appData.activity_records });
        }
    } else if (tabId === "workouts") {
        if (!appData.workouts_loaded && !appData.workouts_loading) loadMetric("workouts");
        else if (appData.workouts_loading) showLoadingSpinner("workouts");
        else if (appData.workouts_loaded) {
            populateWorkoutsTab(appData.exercise_sessions);
        }
    } else if (tabId === "vitals") {
        if (!appData.vitals_loaded && !appData.vitals_loading) loadMetric("vitals");
        else if (appData.vitals_loading) showLoadingSpinner("vitals");
        else if (appData.vitals_loaded) {
            populateVitalsTab(appData);
            renderVitalsCharts(appData);
        }
    } else if (tabId === "nutrition") {
        if (!appData.nutrition_loaded && !appData.nutrition_loading) loadMetric("nutrition");
        else if (appData.nutrition_loading) showLoadingSpinner("nutrition");
        else if (appData.nutrition_loaded) {
            populateNutritionTab(appData.nutrition_records);
            renderNutritionCharts(appData);
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

// Fetch general configuration status
async function refreshStatus() {
    try {
        const res = await fetch("/api/status");
        appStatus = await res.json();

        const authIcon = document.getElementById("auth-status-icon");
        const authTitle = document.getElementById("auth-status-title");
        const authDesc = document.getElementById("auth-status-desc");
        
        const btnGoogle = document.getElementById("btn-google-connect");
        const btnForceSync = document.getElementById("btn-force-sync");
        const btnPurge = document.getElementById("btn-purge-db");
        
        const welcomeHero = document.getElementById("welcome-hero");
        const userNameEl = document.getElementById("user-name");
        const avatarImg = document.getElementById("avatar-img");
        const avatarBadge = document.getElementById("avatar-badge");

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
            const initial = (appStatus.user_name || appStatus.user_email || "U")[0].toUpperCase();
            avatarBadge.textContent = initial;
            if (appStatus.user_picture) {
                avatarImg.referrerPolicy = "no-referrer";
                avatarImg.onerror = () => {
                    avatarImg.style.display = "none";
                    avatarBadge.style.display = "flex";
                };
                avatarImg.onload = () => {
                    avatarImg.style.display = "inline-block";
                    avatarBadge.style.display = "none";
                };
                avatarImg.src = appStatus.user_picture;
            } else {
                avatarImg.style.display = "none";
                avatarBadge.style.display = "flex";
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
            resetAppData();
        }

        lucide.createIcons();
    } catch (e) {
        showToast("Failed to load server status endpoint", true);
    }
}


// Redirect User to Google Consent screen (supports incremental scope authorization)
async function redirectToGoogle(scope = '') {
    try {
        let url = "/api/auth-url";
        if (scope && typeof scope === "string") {
            url += "?scope=" + encodeURIComponent(scope);
        }
        const res = await fetch(url);
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

    try {
        localStorage.removeItem("pulse_app_cache");
        localStorage.removeItem(CACHE_KEY);
    } catch (e) {}

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
        await fetch("/api/cache/clear", { method: "POST" }).catch(() => {});
        resetAppData();
        showToast("Server & browser cache cleared! Health data will lazy load when you visit each tab.");
    } catch (e) {
        showToast("Failed to clear cache", true);
    } finally {
        if (btnSettings) btnSettings.disabled = false;
    }
}

// Fetch stats and render UI
async function loadStats(forceRefresh = false) {
    if (!appStatus.oauth_connected) {
        return;
    }

    const activeTabPane = document.querySelector(".tab-pane.active");
    const activeTabId = activeTabPane ? activeTabPane.id.replace("tab-", "") : "overview";
    
    if (!forceRefresh && loadCache()) {
        const raw = localStorage.getItem(CACHE_KEY);
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
            populateHeartTable(appData.rhr_records, appData.hrv_records, appData.vo2_max_records, appData.hr_zone_records);
            renderHeartCharts(appData);
        }
        if (appData.activity_loaded) {
            populateActivityTable(appData.activity_records);
            renderActivityCharts({ activity_records: appData.activity_records });
        }
        if (appData.workouts_loaded) {
            populateWorkoutsTab(appData.exercise_sessions);
        }
        if (appData.vitals_loaded) {
            populateVitalsTab(appData);
            renderVitalsCharts(appData);
        }
        if (appData.nutrition_loaded) {
            populateNutritionTab(appData.nutrition_records);
            renderNutritionCharts(appData);
        }

        if (!["overview", "settings"].includes(activeTabId) && !appData[`${activeTabId}_loaded`]) {
            await loadMetric(activeTabId);
        }
        return;
    }

    if (activeTabId !== "settings") {
        await loadMetric(activeTabId);
    } else {
        await loadMetric("overview");
    }
}

function showLoadingSpinner(metric) {
    const chartMap = {
        overview: ["chart-overview-sleep", "chart-overview-heart"],
        sleep: ["chart-sleep-durations", "chart-sleep-stages"],
        heart: ["chart-hrv", "chart-rhr", "chart-hr-zones", "chart-vo2-max"],
        rhr: ["chart-hrv", "chart-rhr"],
        hrv: ["chart-hrv", "chart-rhr"],
        activity: ["chart-steps", "chart-calories", "chart-distance", "chart-floors"],
        workouts: ["chart-workout-profile"],
        vitals: ["chart-spo2", "chart-respiratory", "chart-skin-temp", "chart-body-comp"],
        nutrition: ["chart-hydration", "chart-macros"]
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
            tbody.innerHTML = `<tr><td colspan="8" style="text-align: center; padding: 3rem 0;"><div style="display: inline-flex; align-items: center; gap: 10px; color: var(--text-muted); font-weight: 500;"><i data-lucide="loader" class="spin" style="width: 22px; height: 22px; color: var(--primary-color);"></i> Loading sleep history from Google Health...</div></td></tr>`;
        }
    } else if (metric === "heart" || metric === "rhr" || metric === "hrv") {
        const tbody = document.querySelector("#heart-history-table tbody");
        if (tbody && (!appData.rhr_records || appData.rhr_records.length === 0)) {
            tbody.innerHTML = `<tr><td colspan="7" style="text-align: center; padding: 3rem 0;"><div style="display: inline-flex; align-items: center; gap: 10px; color: var(--text-muted); font-weight: 500;"><i data-lucide="loader" class="spin" style="width: 22px; height: 22px; color: var(--primary-color);"></i> Loading heart vitals from Google Health...</div></td></tr>`;
        }
    } else if (metric === "activity") {
        const tbody = document.querySelector("#activity-history-table tbody");
        if (tbody && (!appData.activity_records || appData.activity_records.length === 0)) {
            tbody.innerHTML = `<tr><td colspan="7" style="text-align: center; padding: 3rem 0;"><div style="display: inline-flex; align-items: center; gap: 10px; color: var(--text-muted); font-weight: 500;"><i data-lucide="loader" class="spin" style="width: 22px; height: 22px; color: var(--primary-color);"></i> Loading activity log from Google Health...</div></td></tr>`;
        }
    } else if (metric === "workouts") {
        const tbody = document.querySelector("#workouts-history-table tbody");
        if (tbody && (!appData.exercise_sessions || appData.exercise_sessions.length === 0)) {
            tbody.innerHTML = `<tr><td colspan="9" style="text-align: center; padding: 3rem 0;"><div style="display: inline-flex; align-items: center; gap: 10px; color: var(--text-muted); font-weight: 500;"><i data-lucide="loader" class="spin" style="width: 22px; height: 22px; color: var(--primary-color);"></i> Loading workouts & GPS routes from Google Health...</div></td></tr>`;
        }
    } else if (metric === "vitals") {
        const tbody = document.querySelector("#vitals-history-table tbody");
        if (tbody && (!appData.spo2_records || appData.spo2_records.length === 0)) {
            tbody.innerHTML = `<tr><td colspan="6" style="text-align: center; padding: 3rem 0;"><div style="display: inline-flex; align-items: center; gap: 10px; color: var(--text-muted); font-weight: 500;"><i data-lucide="loader" class="spin" style="width: 22px; height: 22px; color: var(--primary-color);"></i> Loading SpO2, respiratory rate & body composition...</div></td></tr>`;
        }
    } else if (metric === "nutrition") {
        const tbody = document.querySelector("#nutrition-history-table tbody");
        if (tbody && (!appData.nutrition_records || appData.nutrition_records.length === 0)) {
            tbody.innerHTML = `<tr><td colspan="7" style="text-align: center; padding: 3rem 0;"><div style="display: inline-flex; align-items: center; gap: 10px; color: var(--text-muted); font-weight: 500;"><i data-lucide="loader" class="spin" style="width: 22px; height: 22px; color: var(--primary-color);"></i> Loading hydration & nutrition logs...</div></td></tr>`;
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
        heart: ["chart-hrv", "chart-rhr", "chart-hr-zones", "chart-vo2-max"],
        rhr: ["chart-hrv", "chart-rhr"],
        hrv: ["chart-hrv", "chart-rhr"],
        activity: ["chart-steps", "chart-calories", "chart-distance", "chart-floors"],
        workouts: ["chart-workout-profile"],
        vitals: ["chart-spo2", "chart-respiratory", "chart-skin-temp", "chart-body-comp"],
        nutrition: ["chart-hydration", "chart-macros"]
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

function handleMissingScopes(metric, missingScopes) {
    if (!missingScopes || !Array.isArray(missingScopes) || missingScopes.length === 0) return;
    
    let targetScope = "";
    let scopeName = "";
    if ((metric === "sleep" || metric === "overview") && missingScopes.includes("sleep")) {
        targetScope = "sleep";
        scopeName = "Sleep Analysis";
    } else if ((metric === "heart" || metric === "rhr" || metric === "hrv" || metric === "overview") && missingScopes.includes("heart")) {
        targetScope = "heart";
        scopeName = "Heart Health";
    } else if ((metric === "activity" || metric === "overview") && missingScopes.includes("activity")) {
        targetScope = "activity";
        scopeName = "Activity & Fitness";
    } else if ((metric === "workouts" || metric === "overview") && missingScopes.includes("workouts")) {
        targetScope = "workouts";
        scopeName = "Workouts & GPS Location";
    } else if ((metric === "vitals" || metric === "overview") && missingScopes.includes("vitals")) {
        targetScope = "vitals";
        scopeName = "Vitals & Body Measurements";
    } else if ((metric === "nutrition" || metric === "overview") && missingScopes.includes("nutrition")) {
        targetScope = "nutrition";
        scopeName = "Nutrition & Hydration";
    }

    if (targetScope && appStatus.oauth_connected) {
        showToast(`Permission required: Grant read permission for ${scopeName} data via Settings -> Connect Google Health.`, true);
    }
}

async function loadMetric(metric, pageToken = "") {
    if (!appStatus.oauth_connected) return;
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
        if (data && data.missing_scopes && data.missing_scopes.length > 0) {
            handleMissingScopes(metric, data.missing_scopes);
        }

        if (metric === "overview") {
            appData.overview_loaded = true;
            if (!appData.sleep_loaded) appData.sleep_sessions = data.sleep_sessions || [];
            if (!appData.heart_loaded) {
                appData.rhr_records = data.rhr_records || [];
                appData.hrv_records = data.hrv_records || [];
                appData.hr_zone_records = data.hr_zone_records || [];
                appData.vo2_max_records = data.vo2_max_records || [];
            }
            if (!appData.activity_loaded) appData.activity_records = data.activity_records || [];
            if (!appData.workouts_loaded) appData.exercise_sessions = data.exercise_sessions || [];
            if (!appData.vitals_loaded) {
                appData.spo2_records = data.spo2_records || [];
                appData.respiratory_records = data.respiratory_records || [];
                appData.sleep_temp_records = data.sleep_temp_records || [];
                appData.body_records = data.body_records || [];
            }
            if (!appData.nutrition_loaded) appData.nutrition_records = data.nutrition_records || [];
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
            if (!pageToken) {
                appData.hr_zone_records = data.hr_zone_records || [];
                appData.vo2_max_records = data.vo2_max_records || [];
            }
            populateHeartTable(appData.rhr_records, appData.hrv_records, appData.vo2_max_records, appData.hr_zone_records);
            appData.heart_loaded = true;
            renderHeartCharts(appData);
        } else if (metric === "activity") {
            if (pageToken) {
                const map = new Map();
                (appData.activity_records || []).forEach(r => map.set(r.date, { ...r }));
                (data.activity_records || []).forEach(r => {
                    if (map.has(r.date)) {
                        const existing = map.get(r.date);
                        existing.steps = Math.max(existing.steps || 0, r.steps || 0);
                        existing.calories_burned = Math.max(existing.calories_burned || 0, r.calories_burned || 0);
                        existing.basal_calories = Math.max(existing.basal_calories || 0, r.basal_calories || 0);
                        existing.active_minutes = Math.max(existing.active_minutes || 0, r.active_minutes || 0);
                        existing.distance_km = Math.max(existing.distance_km || 0, r.distance_km || 0);
                        existing.floors = Math.max(existing.floors || 0, r.floors || 0);
                    } else {
                        map.set(r.date, { ...r });
                    }
                });
                appData.activity_records = Array.from(map.values()).sort((a, b) => (a.date < b.date ? 1 : -1));
            } else {
                appData.activity_records = data.activity_records || [];
            }
            appData.activity_next_page_token = data.activity_next_page_token || "";
            appData.activity_loaded = true;
            populateActivityTable(appData.activity_records);
            if (!pageToken) {
                renderActivityCharts({ activity_records: appData.activity_records });
            }
        } else if (metric === "workouts") {
            if (pageToken) {
                appData.exercise_sessions = appData.exercise_sessions.concat(data.exercise_sessions || []);
            } else {
                appData.exercise_sessions = data.exercise_sessions || [];
            }
            appData.exercise_next_page_token = data.exercise_next_page_token || "";
            appData.workouts_loaded = true;
            populateWorkoutsTab(appData.exercise_sessions);
        } else if (metric === "vitals") {
            appData.spo2_records = data.spo2_records || [];
            appData.respiratory_records = data.respiratory_records || [];
            appData.sleep_temp_records = data.sleep_temp_records || [];
            appData.body_records = data.body_records || [];
            appData.vitals_loaded = true;
            populateVitalsTab(appData);
            renderVitalsCharts(appData);
        } else if (metric === "nutrition") {
            appData.nutrition_records = data.nutrition_records || [];
            appData.nutrition_loaded = true;
            populateNutritionTab(appData.nutrition_records);
            renderNutritionCharts(appData);
        }
        saveCache();
    } catch (e) {
        showToast(`Failed to load ${metric} data`, true);
    } finally {
        appData[`${metric}_loading`] = false;
        hideLoadingSpinner(metric);
        if (!appData[`${metric}_loaded`]) {
            if (metric === "sleep") populateSleepTable(appData.sleep_sessions || []);
            else if (metric === "heart") populateHeartTable(appData.rhr_records || [], appData.hrv_records || [], appData.vo2_max_records || [], appData.hr_zone_records || []);
            else if (metric === "activity") populateActivityTable(appData.activity_records || []);
            else if (metric === "workouts") populateWorkoutsTab(appData.exercise_sessions || []);
            else if (metric === "vitals") populateVitalsTab(appData);
            else if (metric === "nutrition") populateNutritionTab(appData.nutrition_records || []);
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

function normalizeSleepStage(rawType) {
    const t = (rawType || "").toUpperCase();
    if (t.includes("DEEP")) return "DEEP";
    if (t.includes("REM")) return "REM";
    if (t.includes("AWAKE") || t.includes("RESTLESS") || t.includes("WAKE")) return "AWAKE";
    return "LIGHT";
}

function formatHoursMins(minsTotal) {
    const hrs = Math.floor(minsTotal / 60);
    const mins = minsTotal % 60;
    return hrs > 0 ? `${hrs}h ${mins}m` : `${mins}m`;
}

function populateSleepTable(sessions) {
    const sleepTbody = document.querySelector("#sleep-history-table tbody");
    if (!sleepTbody) return;
    sleepTbody.innerHTML = "";
    if (sessions && sessions.length > 0) {
        sessions.forEach(s => {
            const tr = document.createElement("tr");
            tr.style.cursor = "pointer";
            tr.id = `sleep-row-${s.id}`;
            tr.onclick = () => selectSleepSession(s);

            const st = new Date(s.start_time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
            const et = new Date(s.end_time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
            const sDate = new Date(s.end_time || s.start_time).toLocaleDateString([], { month: 'short', day: 'numeric', year: 'numeric' });
            
            const hrs = Math.floor(s.duration_minutes / 60);
            const mins = s.duration_minutes % 60;
            const eff = s.duration_minutes > 0 ? Math.round((s.minutes_asleep / s.duration_minutes) * 100) : 0;
            const effClass = eff >= 85 ? "badge badge-optimal" : (eff >= 75 ? "badge badge-normal" : "badge badge-warning");
            
            tr.innerHTML = `
                <td><strong>${sDate}</strong></td>
                <td>${st}</td>
                <td>${et}</td>
                <td>${hrs}h ${mins}m</td>
                <td><strong>${formatHoursMins(s.minutes_asleep)}</strong></td>
                <td>${s.minutes_awake}m</td>
                <td><span class="${effClass}">${eff}%</span></td>
                <td><button class="btn btn-secondary" style="padding: 5px 11px; font-size: 0.78rem;" onclick="event.stopPropagation(); selectSleepSessionById('${s.id}')"><i data-lucide="bar-chart-2" style="width: 13px; height: 13px;"></i> View Stages</button></td>
            `;
            sleepTbody.appendChild(tr);
        });
        if (typeof lucide !== "undefined" && lucide.createIcons) lucide.createIcons();

        const targetSession = sessions.find(s => s.id === selectedSleepId) || sessions[0];
        if (targetSession) {
            selectSleepSession(targetSession);
        }
    } else {
        sleepTbody.innerHTML = `<tr><td colspan="8" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
    }
}

function selectSleepSessionById(id) {
    const s = (appData.sleep_sessions || []).find(item => item.id === id);
    if (s) selectSleepSession(s);
}

function selectSleepSession(session) {
    if (!session) return;
    selectedSleepId = session.id;

    // Highlight selected row
    document.querySelectorAll("#sleep-history-table tbody tr").forEach(tr => {
        tr.style.background = tr.id === `sleep-row-${session.id}` ? "rgba(139, 92, 246, 0.12)" : "";
    });

    const titleEl = document.getElementById("sleep-detail-title");
    const subEl = document.getElementById("sleep-detail-subtitle");
    const badgeEl = document.getElementById("sleep-efficiency-badge");

    const wakeDateStr = new Date(session.end_time || session.start_time).toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' });
    const bedTimeStr = new Date(session.start_time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    const wakeTimeStr = new Date(session.end_time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    const eff = session.duration_minutes > 0 ? Math.round((session.minutes_asleep / session.duration_minutes) * 100) : 0;

    if (titleEl) titleEl.textContent = `Sleep Session — ${wakeDateStr}`;
    if (subEl) {
        subEl.textContent = `${bedTimeStr} → ${wakeTimeStr} • ${formatHoursMins(session.duration_minutes)} in bed • ${formatHoursMins(session.minutes_asleep)} asleep • ${session.minutes_awake}m awake`;
    }
    if (badgeEl) {
        badgeEl.textContent = `${eff}% Sleep Efficiency`;
        badgeEl.className = eff >= 85 ? "badge badge-optimal" : (eff >= 75 ? "badge badge-normal" : "badge badge-warning");
    }

    // Calculate stage totals for this session
    let deepMins = 0, remMins = 0, lightMins = 0, awakeMins = 0;
    const rawStages = (session.stages || []).slice().sort((a, b) => (a.start_time > b.start_time ? 1 : -1));

    if (rawStages.length > 0) {
        rawStages.forEach(st => {
            const cat = normalizeSleepStage(st.stage_type);
            let dur = st.duration_minutes || 0;
            if (dur <= 0 && st.start_time && st.end_time) {
                dur = Math.max(1, Math.round((new Date(st.end_time) - new Date(st.start_time)) / 60000));
            }
            if (cat === "DEEP") deepMins += dur;
            else if (cat === "REM") remMins += dur;
            else if (cat === "AWAKE") awakeMins += dur;
            else lightMins += dur;
        });
    } else {
        remMins = Math.round(session.minutes_asleep * 0.20);
        deepMins = Math.round(session.minutes_asleep * 0.15);
        lightMins = Math.max(0, session.minutes_asleep - remMins - deepMins);
        awakeMins = session.minutes_awake || 0;
    }

    const totalStageMins = Math.max(1, deepMins + remMins + lightMins + awakeMins);
    const deepPct = Math.round((deepMins / totalStageMins) * 100);
    const remPct = Math.round((remMins / totalStageMins) * 100);
    const lightPct = Math.round((lightMins / totalStageMins) * 100);
    const awakePct = Math.round((awakeMins / totalStageMins) * 100);

    const summaryEl = document.getElementById("sleep-stages-summary");
    if (summaryEl) {
        summaryEl.textContent = `Deep ${formatHoursMins(deepMins)} (${deepPct}%) • REM ${formatHoursMins(remMins)} (${remPct}%) • Light ${formatHoursMins(lightMins)} (${lightPct}%) • Awake ${formatHoursMins(awakeMins)} (${awakePct}%)`;
    }
    const barEl = document.getElementById("sleep-stages-bar");
    if (barEl) {
        barEl.innerHTML = `
            <div style="width: ${(deepMins / totalStageMins) * 100}%; background: #a855f7;" title="Deep: ${formatHoursMins(deepMins)} (${deepPct}%)"></div>
            <div style="width: ${(remMins / totalStageMins) * 100}%; background: #38bdf8;" title="REM: ${formatHoursMins(remMins)} (${remPct}%)"></div>
            <div style="width: ${(lightMins / totalStageMins) * 100}%; background: #818cf8;" title="Light: ${formatHoursMins(lightMins)} (${lightPct}%)"></div>
            <div style="width: ${(awakeMins / totalStageMins) * 100}%; background: #f43f5e;" title="Awake: ${formatHoursMins(awakeMins)} (${awakePct}%)"></div>
        `;
    }

    // 1. Render Stepped Hypnogram Chart
    const stageLevelMap = { DEEP: 1, LIGHT: 2, REM: 3, AWAKE: 4 };
    const stageNameMap = { 1: "Deep Sleep", 2: "Light Sleep", 3: "REM Sleep", 4: "Awake" };
    const stageColorMap = { 1: "#a855f7", 2: "#818cf8", 3: "#38bdf8", 4: "#f43f5e" };

    const hypnoLabels = [];
    const hypnoValues = [];

    if (rawStages.length > 0) {
        const startMs = new Date(rawStages[0].start_time || session.start_time).getTime();
        const endMs = new Date(rawStages[rawStages.length - 1].end_time || session.end_time).getTime();
        const stepMs = 2 * 60 * 1000; // 2-minute intervals for true time-proportional x-axis

        const parsedIntervals = rawStages.map(st => ({
            s: new Date(st.start_time).getTime(),
            e: new Date(st.end_time).getTime(),
            lvl: stageLevelMap[normalizeSleepStage(st.stage_type)] || 2
        }));

        for (let t = startMs; t <= endMs; t += stepMs) {
            hypnoLabels.push(new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
            const active = parsedIntervals.find(iv => t >= iv.s && t <= iv.e);
            hypnoValues.push(active ? active.lvl : 2);
        }
    } else {
        hypnoLabels.push(bedTimeStr, wakeTimeStr);
        hypnoValues.push(2, 2);
    }

    renderChart("chart-sleep-hypnogram", {
        type: 'line',
        data: {
            labels: hypnoLabels,
            datasets: [{
                label: 'Sleep Stage',
                data: hypnoValues,
                stepped: true,
                borderWidth: 3,
                pointRadius: 0,
                pointHoverRadius: 5,
                fill: true,
                backgroundColor: 'rgba(129, 140, 248, 0.08)',
                borderColor: '#818cf8',
                segment: {
                    borderColor: ctx => stageColorMap[ctx.p0.parsed.y] || '#818cf8'
                }
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: { mode: 'index', intersect: false },
            plugins: {
                legend: { display: false },
                tooltip: {
                    callbacks: {
                        label: ctx => ` ${stageNameMap[ctx.parsed.y] || 'Sleep'}`
                    }
                }
            },
            scales: {
                x: {
                    grid: { color: 'rgba(255,255,255,0.03)' },
                    ticks: { color: '#94a3b8', maxTicksLimit: 9 }
                },
                y: {
                    min: 0.6,
                    max: 4.4,
                    grid: { color: 'rgba(255,255,255,0.04)' },
                    ticks: {
                        color: '#e2e8f0',
                        stepSize: 1,
                        callback: val => ({ 1: 'Deep', 2: 'Light', 3: 'REM', 4: 'Awake' }[val] || '')
                    }
                }
            }
        }
    });

    // 2. Render Session Stage Proportions Horizontal Bar Chart
    renderChart("chart-sleep-session-stages", {
        type: 'bar',
        data: {
            labels: [
                `Deep (${deepPct}%)`,
                `REM (${remPct}%)`,
                `Light (${lightPct}%)`,
                `Awake (${awakePct}%)`
            ],
            datasets: [{
                label: 'Minutes',
                data: [deepMins, remMins, lightMins, awakeMins],
                backgroundColor: ['#a855f7', '#38bdf8', '#818cf8', '#f43f5e'],
                borderRadius: 6,
                barThickness: 22
            }]
        },
        options: {
            indexAxis: 'y',
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: { display: false },
                tooltip: {
                    callbacks: {
                        label: ctx => ` ${formatHoursMins(ctx.parsed.x)} (${ctx.parsed.x} mins)`
                    }
                }
            },
            scales: {
                x: {
                    beginAtZero: true,
                    grid: { color: 'rgba(255,255,255,0.03)' },
                    ticks: { color: '#94a3b8' },
                    title: { display: true, text: 'Minutes', color: '#94a3b8' }
                },
                y: {
                    grid: { display: false },
                    ticks: { color: '#e2e8f0', font: { weight: '600' } }
                }
            }
        }
    });
}

function populateHeartTable(rhrRecords, hrvRecords, vo2Records, zoneRecords) {
    const tbody = document.querySelector("#heart-history-table tbody");
    if (!tbody) return;
    tbody.innerHTML = "";

    const rhrMap = {};
    (rhrRecords || []).forEach(r => rhrMap[r.date] = r);
    const hrvMap = {};
    (hrvRecords || []).forEach(r => hrvMap[r.date] = r);
    const vo2Map = {};
    (vo2Records || []).forEach(r => vo2Map[r.date] = r);
    const zoneMap = {};
    (zoneRecords || []).forEach(r => zoneMap[r.date] = r);

    const dates = Array.from(new Set([
        ...Object.keys(rhrMap),
        ...Object.keys(hrvMap),
        ...Object.keys(vo2Map),
        ...Object.keys(zoneMap)
    ])).sort().reverse();

    if (dates.length > 0) {
        dates.forEach(d => {
            const rhr = rhrMap[d];
            const hrv = hrvMap[d];
            const vo2 = vo2Map[d];
            const zn = zoneMap[d];
            const tr = document.createElement("tr");
            const rhrVal = rhr ? `<strong>${rhr.beats_per_minute}</strong> BPM` : "--";
            const hrvVal = hrv ? `<strong>${Math.round(hrv.avg_hrv_ms)}</strong> ms` : "--";
            const deepVal = (hrv && hrv.deep_sleep_rmssd) ? `${Math.round(hrv.deep_sleep_rmssd)} ms` : "--";
            const vo2Val = vo2 ? `<strong>${vo2.vo2_max}</strong> mL/kg/min` : "--";
            const znVal = zn ? `${zn.moderate_minutes + zn.vigorous_minutes + zn.peak_minutes}m (${zn.peak_minutes}m peak)` : "--";
            
            let status = `<span class="badge badge-normal">Normal</span>`;
            if (rhr && rhr.beats_per_minute > 80) status = `<span class="badge badge-warning">Elevated RHR</span>`;
            else if (hrv && hrv.avg_hrv_ms > 60) status = `<span class="badge badge-optimal">Optimal HRV</span>`;

            tr.innerHTML = `
                <td>${d}</td>
                <td>${rhrVal}</td>
                <td>${hrvVal}</td>
                <td>${deepVal}</td>
                <td>${vo2Val}</td>
                <td>${znVal}</td>
                <td>${status}</td>
            `;
            tbody.appendChild(tr);
        });
    } else {
        tbody.innerHTML = `<tr><td colspan="7" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
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
            const distStr = a.distance_km ? `${a.distance_km.toFixed(2)} km` : "--";
            const floorsStr = a.floors !== undefined ? `${a.floors} fl` : "--";
            const totalCals = a.basal_calories ? `${a.calories_burned} / ${(a.calories_burned + a.basal_calories).toLocaleString()} kcal` : `${a.calories_burned} kcal`;
            tr.innerHTML = `
                <td>${a.date}</td>
                <td><strong>${a.steps.toLocaleString()}</strong></td>
                <td>${distStr}</td>
                <td>${floorsStr}</td>
                <td>${totalCals}</td>
                <td>${a.active_minutes} mins</td>
                <td><span class="${badgeClass}" style="gap: 6px;"><i data-lucide="${iconName}" style="width: 15px; height: 15px;"></i>${goalText} (${Math.round((a.steps / 10000) * 100)}%)</span></td>
            `;
            actTbody.appendChild(tr);
        });
        if (typeof lucide !== "undefined" && lucide.createIcons) lucide.createIcons();
    } else {
        actTbody.innerHTML = `<tr><td colspan="7" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
    }
}

const lastScrollPositions = new WeakMap();
let lastWindowScrollY = 0;

function checkInfiniteScroll(scrollTarget = window) {
    const activeTabPane = document.querySelector(".tab-pane.active");
    if (!activeTabPane) return;

    const tabId = activeTabPane.id.replace("tab-", "");
    let shouldLoadMore = false;

    if (scrollTarget === window || scrollTarget === document) {
        const currentY = window.scrollY;
        const scrollingDown = currentY > lastWindowScrollY;
        lastWindowScrollY = currentY;
        if (!scrollingDown) return;

        // If the active tab has a scrollable table container, rely on the table scroll instead of window scroll
        const hasScrollableTable = Array.from(activeTabPane.querySelectorAll(".table-responsive, .table-container"))
            .some(c => c.scrollHeight > c.clientHeight + 20);
        if (!hasScrollableTable && (window.innerHeight + currentY) >= (document.documentElement.scrollHeight - 150)) {
            shouldLoadMore = true;
        }
    } else if (scrollTarget && scrollTarget.classList && (scrollTarget.classList.contains("table-responsive") || scrollTarget.classList.contains("table-container"))) {
        if (!activeTabPane.contains(scrollTarget)) return;
        const prevTop = lastScrollPositions.get(scrollTarget) || 0;
        const currentTop = scrollTarget.scrollTop;
        const scrollingDown = currentTop > prevTop;
        lastScrollPositions.set(scrollTarget, currentTop);

        if (!scrollingDown) return;
        if (scrollTarget.scrollHeight > scrollTarget.clientHeight && (currentTop + scrollTarget.clientHeight) >= (scrollTarget.scrollHeight - 80)) {
            shouldLoadMore = true;
        }
    }

    if (shouldLoadMore) {
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
        } else if (tabId === "workouts" && appData.exercise_next_page_token && !appData.workouts_loading) {
            showToast("Loading more workout sessions...");
            loadMetric("workouts", appData.exercise_next_page_token);
        }
    }
}

window.addEventListener("scroll", () => checkInfiniteScroll(window), { passive: true });
document.addEventListener("scroll", function(e) {
    if (e.target && e.target.classList && (e.target.classList.contains("table-responsive") || e.target.classList.contains("table-container"))) {
        checkInfiniteScroll(e.target);
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
                const cat = normalizeSleepStage(st.stage_type);
                if (cat === "REM") rem += dur;
                else if (cat === "DEEP") deep += dur;
                else if (cat === "AWAKE") awake += dur;
                else light += dur;
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

    // 3. Daily Heart Rate Zones Chart
    if (data.hr_zone_records && data.hr_zone_records.length > 0) {
        const zDates = [...data.hr_zone_records].slice(0, 14).reverse();
        const labels = zDates.map(z => new Date(z.date).toLocaleDateString([], { month: 'short', day: 'numeric' }));
        renderChart("chart-hr-zones", {
            type: 'bar',
            data: {
                labels: labels,
                datasets: [
                    { label: 'Light', data: zDates.map(z => z.light_minutes), backgroundColor: '#38bdf8' },
                    { label: 'Moderate (Fat Burn)', data: zDates.map(z => z.moderate_minutes), backgroundColor: '#34d399' },
                    { label: 'Vigorous (Cardio)', data: zDates.map(z => z.vigorous_minutes), backgroundColor: '#f59e0b' },
                    { label: 'Peak', data: zDates.map(z => z.peak_minutes), backgroundColor: '#f43f5e' }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { labels: { color: '#94a3b8' } } },
                scales: {
                    x: { stacked: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                    y: { stacked: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'Minutes', color: '#94a3b8' } }
                }
            }
        });
    }

    // 4. VO2 Max Chart
    if (data.vo2_max_records && data.vo2_max_records.length > 0) {
        const vDates = [...data.vo2_max_records].slice(0, 14).reverse();
        renderChart("chart-vo2-max", {
            type: 'line',
            data: {
                labels: vDates.map(r => new Date(r.date).toLocaleDateString([], { month: 'short', day: 'numeric' })),
                datasets: [{
                    label: 'VO2 Max (mL/kg/min)',
                    data: vDates.map(r => r.vo2_max),
                    borderColor: '#a855f7',
                    backgroundColor: 'rgba(168, 85, 247, 0.08)',
                    borderWidth: 3,
                    pointBackgroundColor: '#a855f7',
                    tension: 0.35,
                    fill: true
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { labels: { color: '#94a3b8' } } },
                scales: {
                    x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                    y: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'mL/kg/min', color: '#a855f7' } }
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

    // Chart 2: Active & Basal Calories burned
    renderChart("chart-calories", {
        type: 'bar',
        data: {
            labels: dates,
            datasets: [
                {
                    label: 'Active Energy (kcal)',
                    data: recentAct.map(a => a.calories_burned),
                    backgroundColor: 'rgba(244, 63, 94, 0.65)',
                    borderColor: '#f43f5e',
                    borderWidth: 1,
                    borderRadius: 4
                },
                {
                    label: 'Basal Energy (kcal)',
                    data: recentAct.map(a => a.basal_calories || 0),
                    backgroundColor: 'rgba(148, 163, 184, 0.25)',
                    borderColor: '#64748b',
                    borderWidth: 1,
                    borderRadius: 4
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: { legend: { labels: { color: '#94a3b8' } } },
            scales: {
                x: { stacked: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                y: { stacked: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } }
            }
        }
    });

    // Chart 3: Daily Distance (km)
    renderChart("chart-distance", {
        type: 'line',
        data: {
            labels: dates,
            datasets: [{
                label: 'Distance (km)',
                data: recentAct.map(a => a.distance_km || 0),
                borderColor: '#38bdf8',
                backgroundColor: 'rgba(56, 189, 248, 0.08)',
                borderWidth: 3,
                pointBackgroundColor: '#38bdf8',
                tension: 0.35,
                fill: true
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: { legend: { labels: { color: '#94a3b8' } } },
            scales: {
                x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                y: { beginAtZero: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'km', color: '#38bdf8' } }
            }
        }
    });

    // Chart 4: Floors Climbed & Active Minutes
    renderChart("chart-floors", {
        type: 'bar',
        data: {
            labels: dates,
            datasets: [
                {
                    label: 'Floors Climbed',
                    data: recentAct.map(a => a.floors || 0),
                    backgroundColor: 'rgba(168, 85, 247, 0.6)',
                    borderColor: '#a855f7',
                    borderWidth: 2,
                    borderRadius: 4,
                    yAxisID: 'y'
                },
                {
                    type: 'line',
                    label: 'Active Minutes',
                    data: recentAct.map(a => a.active_minutes || 0),
                    borderColor: '#34d399',
                    pointBackgroundColor: '#34d399',
                    borderWidth: 2,
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
                y: { type: 'linear', position: 'left', beginAtZero: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'Floors', color: '#a855f7' } },
                y1: { type: 'linear', position: 'right', beginAtZero: true, grid: { drawOnChartArea: false }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'Mins', color: '#34d399' } }
            }
        }
    });
}

// ==========================================
// WORKOUTS & STRAVA-STYLE GPS ROUTE MAPS
// ==========================================

function initWorkoutMap() {
    const mapContainer = document.getElementById("workout-route-map");
    if (!mapContainer || typeof maplibregl === "undefined") return;
    if (workoutMap) return;

    workoutMap = new maplibregl.Map({
        container: "workout-route-map",
        style: "https://tiles.openfreemap.org/styles/dark",
        center: [-0.1657, 51.5073],
        zoom: 12.5,
        attributionControl: true
    });

    workoutMap.addControl(new maplibregl.NavigationControl({ showCompass: true, showZoom: true }), "top-right");

    workoutMap.on("load", () => {
        workoutMapLoaded = true;

        workoutMap.addSource("workout-route", {
            type: "geojson",
            data: { type: "FeatureCollection", features: [] }
        });

        // Outer neon glow layer
        workoutMap.addLayer({
            id: "workout-route-glow",
            type: "line",
            source: "workout-route",
            layout: {
                "line-cap": "round",
                "line-join": "round"
            },
            paint: {
                "line-color": "#f97316",
                "line-width": 10,
                "line-opacity": 0.28,
                "line-blur": 3
            }
        });

        // Core GPS track line layer
        workoutMap.addLayer({
            id: "workout-route-core",
            type: "line",
            source: "workout-route",
            layout: {
                "line-cap": "round",
                "line-join": "round"
            },
            paint: {
                "line-color": "#fb923c",
                "line-width": 4,
                "line-opacity": 0.95
            }
        });

        if (pendingRouteRender) {
            const { workout, routeData } = pendingRouteRender;
            pendingRouteRender = null;
            renderWorkoutRouteOnMap(workout, routeData);
        }
    });
}

function formatPace(durationMins, distKm) {
    if (!distKm || distKm <= 0.05 || !durationMins) return "--";
    const paceDec = durationMins / distKm;
    const pMin = Math.floor(paceDec);
    const pSec = Math.round((paceDec - pMin) * 60);
    return `${pMin}:${pSec < 10 ? '0' : ''}${pSec} /km`;
}

function populateWorkoutsTab(sessions) {
    initWorkoutMap();
    const list = sessions || [];

    // Summary Cards
    const countEl = document.getElementById("workouts-total-count");
    const latestEl = document.getElementById("workouts-latest-name");
    const distEl = document.getElementById("workouts-total-dist");
    const paceEl = document.getElementById("workouts-avg-pace");
    const hrEl = document.getElementById("workouts-avg-hr");
    const azmEl = document.getElementById("workouts-total-azm");
    const calsEl = document.getElementById("workouts-total-cals");
    const minsEl = document.getElementById("workouts-total-mins");

    let totalDist = 0, totalCals = 0, totalMins = 0, totalAzm = 0, hrSum = 0, hrCnt = 0;
    list.forEach(w => {
        totalDist += (w.distance_km || 0);
        totalCals += (w.calories_kcal || 0);
        totalMins += (w.duration_minutes || 0);
        totalAzm += (w.active_zone_minutes || 0);
        if (w.avg_heart_rate > 0) {
            hrSum += w.avg_heart_rate;
            hrCnt++;
        }
    });

    if (countEl) countEl.textContent = list.length;
    if (latestEl) latestEl.textContent = list.length > 0 ? `Latest: ${list[0].display_name}` : "No workouts recorded";
    if (distEl) distEl.innerHTML = `${totalDist.toFixed(1)} <span class="unit">km</span>`;
    if (paceEl) paceEl.textContent = totalDist > 0 ? `Avg Pace: ${formatPace(totalMins, totalDist)}` : "Outdoor & indoor sessions";
    if (hrEl) hrEl.innerHTML = `${hrCnt > 0 ? Math.round(hrSum / hrCnt) : '--'} <span class="unit">BPM</span>`;
    if (azmEl) azmEl.textContent = `${totalAzm} Active Zone Mins`;
    if (calsEl) calsEl.innerHTML = `${totalCals.toLocaleString()} <span class="unit">kcal</span>`;
    if (minsEl) minsEl.textContent = `${totalMins} total active minutes`;

    // Table
    const tbody = document.querySelector("#workouts-history-table tbody");
    if (!tbody) return;
    tbody.innerHTML = "";

    if (list.length === 0) {
        tbody.innerHTML = `<tr><td colspan="9" style="text-align: center; color: var(--text-muted);">No workout sessions found.</td></tr>`;
        return;
    }

    list.forEach((w, idx) => {
        const tr = document.createElement("tr");
        tr.style.cursor = "pointer";
        tr.id = `workout-row-${w.id}`;
        tr.onclick = () => selectWorkoutRoute(w);

        const dt = new Date(w.start_time);
        const dateStr = dt.toLocaleDateString([], { month: 'short', day: 'numeric', year: 'numeric' });
        const timeStr = dt.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        const distStr = w.distance_km > 0 ? `<strong>${w.distance_km.toFixed(2)} km</strong>` : "--";
        const paceStr = formatPace(w.duration_minutes, w.distance_km);
        const hrStr = w.avg_heart_rate > 0 ? `${w.avg_heart_rate} BPM` : "--";
        const zonesStr = `${w.moderate_zone_mins || 0}m Mod / ${w.vigorous_zone_mins || 0}m Vig / ${w.peak_zone_mins || 0}m Peak`;
        const mapBtn = w.has_gps
            ? `<button class="btn btn-secondary" style="padding: 5px 11px; font-size: 0.78rem;" onclick="event.stopPropagation(); selectWorkoutRouteById('${w.id}')"><i data-lucide="map-pin" style="width: 13px; height: 13px;"></i> View Map</button>`
            : `<span class="badge badge-outline">Indoor</span>`;

        tr.innerHTML = `
            <td>${dateStr} <span style="color: var(--text-muted); font-size: 0.8rem;">${timeStr}</span></td>
            <td><strong>${w.display_name}</strong> <span class="badge badge-outline" style="margin-left: 6px; font-size: 0.72rem;">${w.exercise_type}</span></td>
            <td>${w.duration_minutes}m</td>
            <td>${distStr}</td>
            <td>${paceStr}</td>
            <td>${hrStr}</td>
            <td>${w.calories_kcal} kcal</td>
            <td style="font-size: 0.82rem; color: var(--text-secondary);">${zonesStr}</td>
            <td>${mapBtn}</td>
        `;
        tbody.appendChild(tr);
    });

    if (typeof lucide !== "undefined" && lucide.createIcons) lucide.createIcons();

    // Auto-select first GPS workout (or first workout)
    const firstGps = list.find(w => w.has_gps) || list[0];
    if (firstGps) {
        selectWorkoutRoute(firstGps);
    }
}

function selectWorkoutRouteById(id) {
    const w = (appData.exercise_sessions || []).find(s => s.id === id);
    if (w) selectWorkoutRoute(w);
}

function clearWorkoutMapData() {
    pendingRouteRender = null;
    workoutMarkers.forEach(m => m.remove());
    workoutMarkers = [];
    if (workoutMap && workoutMapLoaded) {
        const routeSource = workoutMap.getSource("workout-route");
        if (routeSource) {
            routeSource.setData({ type: "FeatureCollection", features: [] });
        }
    }
}

function setWorkoutMapLoading(isLoading) {
    const mapContainer = document.getElementById("workout-route-map");
    if (mapContainer) {
        let overlay = mapContainer.querySelector(".chart-loading-overlay");
        if (isLoading) {
            if (!overlay) {
                overlay = document.createElement("div");
                overlay.className = "chart-loading-overlay";
                mapContainer.appendChild(overlay);
            }
            overlay.innerHTML = `<i data-lucide="loader" class="spin" style="width: 28px; height: 28px; color: var(--primary-color);"></i><span>Loading GPS route...</span>`;
            overlay.style.display = "flex";
        } else if (overlay) {
            overlay.remove();
        }
    }

    const profileCanvas = document.getElementById("chart-workout-profile");
    if (profileCanvas && profileCanvas.parentElement) {
        let overlay = profileCanvas.parentElement.querySelector(".chart-loading-overlay");
        if (isLoading) {
            if (activeCharts["chart-workout-profile"]) {
                activeCharts["chart-workout-profile"].destroy();
                delete activeCharts["chart-workout-profile"];
            }
            if (!overlay) {
                overlay = document.createElement("div");
                overlay.className = "chart-loading-overlay";
                profileCanvas.parentElement.appendChild(overlay);
            }
            overlay.innerHTML = `<i data-lucide="loader" class="spin" style="width: 28px; height: 28px; color: var(--primary-color);"></i><span>Loading telemetry...</span>`;
            overlay.style.display = "flex";
        } else if (overlay) {
            overlay.remove();
        }
    }

    if (isLoading && typeof lucide !== "undefined" && lucide.createIcons) {
        lucide.createIcons();
    }
}

async function selectWorkoutRoute(workout) {
    if (!workout) return;
    const requestWorkoutId = workout.id;
    selectedExerciseId = requestWorkoutId;

    // Highlight active row
    document.querySelectorAll("#workouts-history-table tbody tr").forEach(tr => {
        tr.style.background = tr.id === `workout-row-${workout.id}` ? "rgba(139, 92, 246, 0.12)" : "";
    });

    const titleEl = document.getElementById("map-workout-title");
    const subEl = document.getElementById("map-workout-subtitle");
    const badgeEl = document.getElementById("map-gps-badge");
    const dtStr = new Date(workout.start_time).toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' });

    if (titleEl) titleEl.textContent = `${workout.display_name} — ${dtStr}`;
    if (subEl) {
        subEl.textContent = `${workout.duration_minutes} mins • ${workout.distance_km > 0 ? workout.distance_km.toFixed(2) + ' km • Pace ' + formatPace(workout.duration_minutes, workout.distance_km) + ' • ' : ''}${workout.calories_kcal} kcal • Avg HR ${workout.avg_heart_rate || '--'} BPM`;
    }
    if (badgeEl) {
        badgeEl.textContent = "Loading Route...";
        badgeEl.style.color = "var(--text-muted)";
        badgeEl.style.background = "rgba(255, 255, 255, 0.06)";
    }

    // Update HR Zone breakdown bar for this workout
    const lM = workout.light_zone_mins || 0;
    const mM = workout.moderate_zone_mins || 0;
    const vM = workout.vigorous_zone_mins || 0;
    const pM = workout.peak_zone_mins || 0;
    const totZ = Math.max(1, lM + mM + vM + pM);
    const zSumEl = document.getElementById("workout-zones-summary");
    if (zSumEl) {
        zSumEl.textContent = `Light ${lM}m • Moderate ${mM}m • Vigorous ${vM}m • Peak ${pM}m`;
    }
    const zBar = document.getElementById("workout-zones-bar");
    if (zBar) {
        zBar.innerHTML = `
            <div style="width: ${(lM / totZ) * 100}%; background: #38bdf8;" title="Light: ${lM}m"></div>
            <div style="width: ${(mM / totZ) * 100}%; background: #34d399;" title="Moderate: ${mM}m"></div>
            <div style="width: ${(vM / totZ) * 100}%; background: #f59e0b;" title="Vigorous: ${vM}m"></div>
            <div style="width: ${(pM / totZ) * 100}%; background: #f43f5e;" title="Peak: ${pM}m"></div>
        `;
    }

    initWorkoutMap();
    if (workoutMap) workoutMap.resize();

    // Immediately clear previous workout's map track/markers and show loading spinner
    clearWorkoutMapData();
    setWorkoutMapLoading(true);

    try {
        const res = await fetch(`/api/exercise-route?id=${encodeURIComponent(workout.id)}`);
        const routeData = await res.json();
        if (selectedExerciseId !== requestWorkoutId) return;

        setWorkoutMapLoading(false);
        renderWorkoutRouteOnMap(workout, routeData);
        renderWorkoutProfileChart(workout, routeData);
        if (badgeEl) {
            if (routeData && routeData.has_route && routeData.points && routeData.points.length > 0) {
                badgeEl.textContent = `GPS Track (${routeData.points.length} pts)`;
                badgeEl.style.color = "var(--color-emerald)";
                badgeEl.style.background = "rgba(52, 211, 153, 0.15)";
            } else {
                badgeEl.textContent = "Indoor / No GPS Track";
                badgeEl.style.color = "var(--text-muted)";
                badgeEl.style.background = "rgba(255, 255, 255, 0.06)";
            }
        }
    } catch (e) {
        if (selectedExerciseId === requestWorkoutId) {
            setWorkoutMapLoading(false);
            if (badgeEl) {
                badgeEl.textContent = "Route Load Failed";
            }
        }
        console.warn("Failed to load exercise route:", e);
    }
}

function createRouteMarkerElement(color) {
    const el = document.createElement("div");
    el.style.width = "14px";
    el.style.height = "14px";
    el.style.borderRadius = "50%";
    el.style.backgroundColor = color;
    el.style.border = "2px solid #0f172a";
    el.style.boxShadow = `0 0 8px ${color}`;
    el.style.cursor = "pointer";
    return el;
}

function renderWorkoutRouteOnMap(workout, routeData) {
    if (!workoutMap) return;
    if (!workoutMapLoaded) {
        pendingRouteRender = { workout, routeData };
        return;
    }

    // Clear previous markers
    workoutMarkers.forEach(m => m.remove());
    workoutMarkers = [];

    const routeSource = workoutMap.getSource("workout-route");
    if (!routeData || !routeData.has_route || !routeData.points || routeData.points.length === 0) {
        if (routeSource) {
            routeSource.setData({ type: "FeatureCollection", features: [] });
        }
        return;
    }

    const coordinates = routeData.points.map(p => [p.lng, p.lat]);

    if (routeSource) {
        routeSource.setData({
            type: "Feature",
            properties: {},
            geometry: {
                type: "LineString",
                coordinates: coordinates
            }
        });
    }

    // Start Marker (Green)
    const startPt = routeData.points[0];
    const startPopup = new maplibregl.Popup({ offset: 10 }).setHTML(
        `<strong>Start: ${workout.display_name}</strong><br>HR: ${startPt.heart_rate_bpm || '--'} BPM`
    );
    const startMarker = new maplibregl.Marker({ element: createRouteMarkerElement("#34d399") })
        .setLngLat([startPt.lng, startPt.lat])
        .setPopup(startPopup)
        .addTo(workoutMap);
    workoutMarkers.push(startMarker);

    // Finish Marker (Rose/Red)
    const endPt = routeData.points[routeData.points.length - 1];
    const endPopup = new maplibregl.Popup({ offset: 10 }).setHTML(
        `<strong>Finish</strong><br>Distance: ${(endPt.distance_meters / 1000).toFixed(2)} km<br>Final HR: ${endPt.heart_rate_bpm || '--'} BPM`
    );
    const endMarker = new maplibregl.Marker({ element: createRouteMarkerElement("#f43f5e") })
        .setLngLat([endPt.lng, endPt.lat])
        .setPopup(endPopup)
        .addTo(workoutMap);
    workoutMarkers.push(endMarker);

    const bounds = new maplibregl.LngLatBounds();
    coordinates.forEach(coord => bounds.extend(coord));
    workoutMap.fitBounds(bounds, { padding: 36, maxZoom: 16, duration: 600 });
}

function renderWorkoutProfileChart(workout, routeData) {
    const pts = (routeData && routeData.points && routeData.points.length > 0) ? routeData.points : [];
    if (pts.length === 0) {
        renderChart("chart-workout-profile", {
            type: 'bar',
            data: {
                labels: ['Light', 'Moderate', 'Vigorous', 'Peak'],
                datasets: [{
                    label: 'Minutes in Zone',
                    data: [
                        workout.light_zone_mins || 0,
                        workout.moderate_zone_mins || 0,
                        workout.vigorous_zone_mins || 0,
                        workout.peak_zone_mins || 0
                    ],
                    backgroundColor: ['#38bdf8', '#34d399', '#f59e0b', '#f43f5e'],
                    borderRadius: 6
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: {
                    x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                    y: { beginAtZero: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'Minutes', color: '#94a3b8' } }
                }
            }
        });
        return;
    }

    const labels = pts.map(p => `${(p.distance_meters / 1000).toFixed(1)} km`);
    renderChart("chart-workout-profile", {
        type: 'line',
        data: {
            labels: labels,
            datasets: [
                {
                    label: 'Heart Rate (BPM)',
                    data: pts.map(p => p.heart_rate_bpm),
                    borderColor: '#f43f5e',
                    backgroundColor: 'rgba(244, 63, 94, 0.05)',
                    borderWidth: 2.5,
                    pointRadius: 0,
                    tension: 0.3,
                    yAxisID: 'y'
                },
                {
                    label: 'Elevation (m)',
                    data: pts.map(p => p.altitude_meters),
                    borderColor: '#38bdf8',
                    backgroundColor: 'rgba(56, 189, 248, 0.14)',
                    borderWidth: 2,
                    pointRadius: 0,
                    fill: true,
                    tension: 0.35,
                    yAxisID: 'y1'
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: { mode: 'index', intersect: false },
            plugins: { legend: { labels: { color: '#94a3b8' } } },
            scales: {
                x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8', maxTicksLimit: 8 } },
                y: { type: 'linear', position: 'left', grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'BPM', color: '#f43f5e' } },
                y1: { type: 'linear', position: 'right', grid: { drawOnChartArea: false }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'Elev (m)', color: '#38bdf8' } }
            }
        }
    });
}

// ==========================================
// VITALS & BODY COMPOSITION TAB
// ==========================================

function populateVitalsTab(data) {
    const spo2s = data.spo2_records || [];
    const resps = data.respiratory_records || [];
    const temps = data.sleep_temp_records || [];
    const bodys = data.body_records || [];

    // Cards
    const spo2Val = document.getElementById("vitals-spo2-val");
    const spo2Sub = document.getElementById("vitals-spo2-sub");
    if (spo2s.length > 0) {
        const latest = spo2s[0];
        if (spo2Val) spo2Val.textContent = `${latest.avg_percent.toFixed(1)}%`;
        if (spo2Sub) spo2Sub.textContent = `Range: ${latest.min_percent.toFixed(1)}% – ${latest.max_percent.toFixed(1)}% (${latest.date})`;
    }

    const respVal = document.getElementById("vitals-resp-val");
    const respSub = document.getElementById("vitals-resp-sub");
    if (resps.length > 0) {
        const latest = resps[0];
        if (respVal) respVal.innerHTML = `${latest.breaths_per_minute.toFixed(1)} <span class="unit">brpm</span>`;
        if (respSub) respSub.textContent = `Recorded on ${latest.date}`;
    }

    const tempVal = document.getElementById("vitals-temp-val");
    const tempSub = document.getElementById("vitals-temp-sub");
    if (temps.length > 0) {
        const latest = temps[0];
        const sign = latest.delta_c >= 0 ? "+" : "";
        if (tempVal) tempVal.textContent = `${latest.nightly_temp_c.toFixed(2)}°C`;
        if (tempSub) tempSub.textContent = `${sign}${latest.delta_c.toFixed(2)}°C vs ${latest.baseline_c.toFixed(2)}°C baseline`;
    }

    const weightVal = document.getElementById("vitals-weight-val");
    const weightSub = document.getElementById("vitals-weight-sub");
    if (bodys.length > 0) {
        const latest = bodys[0];
        if (weightVal) weightVal.innerHTML = `${latest.weight_kg ? latest.weight_kg.toFixed(1) : '--'} <span class="unit">kg</span>`;
        if (weightSub) weightSub.textContent = latest.body_fat_pct ? `Body Fat: ${latest.body_fat_pct.toFixed(1)}% (${latest.date})` : `Recorded on ${latest.date}`;
    }

    // Table
    const tbody = document.querySelector("#vitals-history-table tbody");
    if (!tbody) return;
    tbody.innerHTML = "";

    const spo2Map = {}, respMap = {}, tempMap = {}, bodyMap = {};
    spo2s.forEach(r => spo2Map[r.date] = r);
    resps.forEach(r => respMap[r.date] = r);
    temps.forEach(r => tempMap[r.date] = r);
    bodys.forEach(r => bodyMap[r.date] = r);

    const dates = Array.from(new Set([
        ...Object.keys(spo2Map),
        ...Object.keys(respMap),
        ...Object.keys(tempMap),
        ...Object.keys(bodyMap)
    ])).sort().reverse();

    if (dates.length === 0) {
        tbody.innerHTML = `<tr><td colspan="6" style="text-align: center; color: var(--text-muted);">No records found.</td></tr>`;
        return;
    }

    dates.forEach(d => {
        const sp = spo2Map[d];
        const rp = respMap[d];
        const tp = tempMap[d];
        const bd = bodyMap[d];
        const tr = document.createElement("tr");

        const spStr = sp ? `<strong>${sp.avg_percent.toFixed(1)}%</strong> <span style="color: var(--text-muted); font-size: 0.8rem;">(${sp.min_percent.toFixed(0)}–${sp.max_percent.toFixed(0)}%)</span>` : "--";
        const rpStr = rp ? `<strong>${rp.breaths_per_minute.toFixed(1)}</strong> brpm` : "--";
        const tpStr = tp ? `${tp.nightly_temp_c.toFixed(2)}°C <span class="badge badge-outline" style="margin-left: 4px;">${tp.delta_c >= 0 ? '+' : ''}${tp.delta_c.toFixed(2)}°C</span>` : "--";
        const wtStr = (bd && bd.weight_kg) ? `<strong>${bd.weight_kg.toFixed(1)} kg</strong>` : "--";
        const bfStr = (bd && bd.body_fat_pct) ? `${bd.body_fat_pct.toFixed(1)}%` : "--";

        tr.innerHTML = `
            <td>${d}</td>
            <td>${spStr}</td>
            <td>${rpStr}</td>
            <td>${tpStr}</td>
            <td>${wtStr}</td>
            <td>${bfStr}</td>
        `;
        tbody.appendChild(tr);
    });
}

function renderVitalsCharts(data) {
    // 1. SpO2 Chart
    if (data.spo2_records && data.spo2_records.length > 0) {
        const recent = [...data.spo2_records].slice(0, 14).reverse();
        const labels = recent.map(r => new Date(r.date).toLocaleDateString([], { month: 'short', day: 'numeric' }));
        renderChart("chart-spo2", {
            type: 'line',
            data: {
                labels: labels,
                datasets: [
                    {
                        label: 'Avg SpO2 (%)',
                        data: recent.map(r => r.avg_percent),
                        borderColor: '#38bdf8',
                        backgroundColor: 'rgba(56, 189, 248, 0.1)',
                        borderWidth: 3,
                        tension: 0.3,
                        fill: true
                    },
                    {
                        label: 'Lower Bound (%)',
                        data: recent.map(r => r.min_percent),
                        borderColor: '#f43f5e',
                        borderDash: [5, 5],
                        borderWidth: 2,
                        tension: 0.3,
                        fill: false
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { labels: { color: '#94a3b8' } } },
                scales: {
                    x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                    y: { suggestedMin: 88, suggestedMax: 100, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'SpO2 %', color: '#38bdf8' } }
                }
            }
        });
    }

    // 2. Respiratory Rate Chart
    if (data.respiratory_records && data.respiratory_records.length > 0) {
        const recent = [...data.respiratory_records].slice(0, 14).reverse();
        renderChart("chart-respiratory", {
            type: 'line',
            data: {
                labels: recent.map(r => new Date(r.date).toLocaleDateString([], { month: 'short', day: 'numeric' })),
                datasets: [{
                    label: 'Breaths / Min',
                    data: recent.map(r => r.breaths_per_minute),
                    borderColor: '#34d399',
                    backgroundColor: 'rgba(52, 211, 153, 0.08)',
                    borderWidth: 3,
                    pointBackgroundColor: '#34d399',
                    tension: 0.35,
                    fill: true
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { labels: { color: '#94a3b8' } } },
                scales: {
                    x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                    y: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'brpm', color: '#34d399' } }
                }
            }
        });
    }

    // 3. Skin Temp Chart
    if (data.sleep_temp_records && data.sleep_temp_records.length > 0) {
        const recent = [...data.sleep_temp_records].slice(0, 14).reverse();
        renderChart("chart-skin-temp", {
            type: 'bar',
            data: {
                labels: recent.map(r => new Date(r.date).toLocaleDateString([], { month: 'short', day: 'numeric' })),
                datasets: [{
                    label: 'Δ from Baseline (°C)',
                    data: recent.map(r => r.delta_c),
                    backgroundColor: recent.map(r => r.delta_c >= 0 ? 'rgba(244, 63, 94, 0.65)' : 'rgba(56, 189, 248, 0.65)'),
                    borderRadius: 4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { labels: { color: '#94a3b8' } } },
                scales: {
                    x: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                    y: { grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'Δ °C', color: '#f43f5e' } }
                }
            }
        });
    }

    // 4. Body Composition Chart
    if (data.body_records && data.body_records.length > 0) {
        const recent = [...data.body_records].slice(0, 14).reverse();
        const labels = recent.map(r => new Date(r.date).toLocaleDateString([], { month: 'short', day: 'numeric' }));
        renderChart("chart-body-comp", {
            type: 'line',
            data: {
                labels: labels,
                datasets: [
                    {
                        label: 'Weight (kg)',
                        data: recent.map(r => r.weight_kg || null),
                        borderColor: '#a855f7',
                        pointBackgroundColor: '#a855f7',
                        borderWidth: 3,
                        tension: 0.3,
                        yAxisID: 'y'
                    },
                    {
                        label: 'Body Fat (%)',
                        data: recent.map(r => r.body_fat_pct || null),
                        borderColor: '#f59e0b',
                        pointBackgroundColor: '#f59e0b',
                        borderWidth: 2.5,
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
                    y: { type: 'linear', position: 'left', grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'kg', color: '#a855f7' } },
                    y1: { type: 'linear', position: 'right', grid: { drawOnChartArea: false }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'Body Fat %', color: '#f59e0b' } }
                }
            }
        });
    }
}

// ==========================================
// NUTRITION & HYDRATION TAB
// ==========================================

function populateNutritionTab(records) {
    const list = records || [];
    const waterVal = document.getElementById("nut-water-val");
    const waterSub = document.getElementById("nut-water-sub");
    const calsVal = document.getElementById("nut-cals-val");
    const calsSub = document.getElementById("nut-cals-sub");
    const protVal = document.getElementById("nut-protein-val");
    const protSub = document.getElementById("nut-protein-sub");
    const macVal = document.getElementById("nut-macros-val");
    const macSub = document.getElementById("nut-macros-sub");

    if (list.length > 0) {
        const latest = list[0];
        if (waterVal) waterVal.innerHTML = `${latest.hydration_ml.toLocaleString()} <span class="unit">ml</span>`;
        if (waterSub) waterSub.textContent = `${Math.round((latest.hydration_ml / 2500) * 100)}% of 2,500 ml goal (${latest.date})`;
        if (calsVal) calsVal.innerHTML = `${latest.calories_in.toLocaleString()} <span class="unit">kcal</span>`;
        if (calsSub) calsSub.textContent = `${latest.meals_logged} meals logged on ${latest.date}`;
        if (protVal) protVal.innerHTML = `${Math.round(latest.protein_g)} <span class="unit">g</span>`;
        if (protSub) protSub.textContent = `${Math.round((latest.protein_g * 4 / Math.max(1, latest.calories_in)) * 100)}% of calories from protein`;
        if (macVal) macVal.textContent = `${Math.round(latest.carbs_g)}g / ${Math.round(latest.fat_g)}g`;
        if (macSub) macSub.textContent = `Carbs / Fat (${latest.date})`;
    }

    const tbody = document.querySelector("#nutrition-history-table tbody");
    if (!tbody) return;
    tbody.innerHTML = "";

    if (list.length === 0) {
        tbody.innerHTML = `<tr><td colspan="7" style="text-align: center; color: var(--text-muted);">No nutrition or hydration records found.</td></tr>`;
        return;
    }

    list.forEach(n => {
        const tr = document.createElement("tr");
        tr.innerHTML = `
            <td>${n.date}</td>
            <td><strong>${n.hydration_ml.toLocaleString()} ml</strong></td>
            <td><strong>${n.calories_in.toLocaleString()} kcal</strong></td>
            <td>${Math.round(n.protein_g)} g</td>
            <td>${Math.round(n.carbs_g)} g</td>
            <td>${Math.round(n.fat_g)} g</td>
            <td><span class="badge badge-outline">${n.meals_logged} meals</span></td>
        `;
        tbody.appendChild(tr);
    });
}

function renderNutritionCharts(data) {
    if (!data.nutrition_records || data.nutrition_records.length === 0) return;

    const recent = [...data.nutrition_records].slice(0, 14).reverse();
    const labels = recent.map(n => new Date(n.date).toLocaleDateString([], { month: 'short', day: 'numeric' }));

    // 1. Hydration Chart
    renderChart("chart-hydration", {
        type: 'bar',
        data: {
            labels: labels,
            datasets: [{
                label: 'Water Intake (ml)',
                data: recent.map(n => n.hydration_ml),
                backgroundColor: 'rgba(56, 189, 248, 0.55)',
                borderColor: '#38bdf8',
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
                y: { beginAtZero: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'ml', color: '#38bdf8' } }
            }
        }
    });

    // 2. Macros Chart
    renderChart("chart-macros", {
        type: 'bar',
        data: {
            labels: labels,
            datasets: [
                { label: 'Protein (g)', data: recent.map(n => n.protein_g), backgroundColor: '#34d399' },
                { label: 'Carbs (g)', data: recent.map(n => n.carbs_g), backgroundColor: '#38bdf8' },
                { label: 'Fat (g)', data: recent.map(n => n.fat_g), backgroundColor: '#a855f7' }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: { legend: { labels: { color: '#94a3b8' } } },
            scales: {
                x: { stacked: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' } },
                y: { stacked: true, grid: { color: 'rgba(255,255,255,0.03)' }, ticks: { color: '#94a3b8' }, title: { display: true, text: 'Grams', color: '#94a3b8' } }
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
