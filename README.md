# Pulse ⚡ Self-Hosted Google Health Dashboard

> **Track, analyze, and visualize your Google Health, Fitbit, and Pixel Watch metrics in real-time with a 100% stateless backend and zero database footprint.**

---

## 🛡️ Why Self-Hosted? (Bypassing the $500/Year CASA Audit Fee)

When applications requesting restricted Google Health scopes (`googlehealth.*.readonly`) are published in **Production** status for public multi-tenant SaaS use, Google strictly enforces an annual **ADA-CASA AL1 (Tier 2) Security Assessment**—which costs **$500 to $2,000+ per year** through third-party auditing labs.

By running **Pulse** as a **self-hosted instance with your own OAuth Client credentials (BYOK)**, your application remains in Google Cloud's **Testing Status**:
* **$0 Cost Forever:** Up to **100 test users/family accounts** can connect completely for free.
* **No CASA Audits:** You are strictly **exempt** from third-party CASA security assessments and annual audit fees.
* **100% Data Ownership:** Your sensitive health records never touch third-party servers, databases, or cloud storage.

---

## ✨ Features

* **⚡ 100% Stateless Backend:** Pulse operates without database servers, persistent storage volumes, or data lakes. When you view your dashboard, data is fetched live from Google Health API and formatted directly in volatile system memory. Once the HTTP request finishes, server buffers are wiped.
* **🎛️ Interactive Web UI Setup:** Don't want to mess with environment variables? Open Pulse in your browser, head to the **Settings** or **Self-Host Guide** tab, paste your Client ID and Secret directly into the UI, and activate your instance on the fly!
* **🔗 Dynamic Redirect URI Detection:** Whether you access Pulse via `http://localhost:8080`, your home network (`http://192.168.1.50:8080`), or a secure reverse-proxy domain (`https://pulse.my-home.local`), Pulse dynamically detects and displays the exact OAuth Callback URI you need.
* **📊 Comprehensive Health Analytics:**
  * **Sleep Stages & Duration:** Deep, REM, light, and awake stage breakdowns.
  * **Heart Health Trends:** Daily Resting Heart Rate (RHR) and Heart Rate Variability (HRV / RMSSD).
  * **Activity & Energy:** Daily step counts, active minutes, and calories burned.

---

## 🚀 Quickstart Deployment

### Option A: Docker Compose (Recommended)

1. Clone or download this repository:
   ```bash
   git clone https://github.com/Jleagle/pulse.git
   cd pulse
   ```

2. Start the container:
   ```bash
   docker compose up -d
   ```

3. Open your browser to **http://localhost:8080** and follow the **2-Minute Setup Guide** in the UI to enter your free Google OAuth keys!

*(Optional: You can also copy `.env.example` to `.env` and pre-configure `CLIENT_ID` and `CLIENT_SECRET` before starting.)*

---

### Option B: Run from Source (Go 1.22+)

1. Build and run directly:
   ```bash
   go run .
   ```
2. Visit **http://localhost:8080** to configure your credentials and link your account.

---

## 📖 2-Minute Guide: Getting Free Google OAuth Keys

Follow these exact steps inside your Google Cloud Console to create keys that never expire and require **no CASA security audit**:

1. **Create a Free Google Cloud Project:**
   * Go to the [Google Cloud Console](https://console.cloud.google.com/), click **New Project**, name it `Pulse Self-Hosted`, and click **Create**.

2. **Enable the Google Health API:**
   * In the left sidebar, navigate to **APIs & Services → Library**.
   * Search for **Google Health API** (or Fitness API) and click **Enable**.

3. **Configure the OAuth Consent Screen (CRITICAL STEP):**
   * Go to **APIs & Services → OAuth consent screen**.
   * Select **External** (or Internal for Google Workspace users) and click **Create**.
   * Set App Name (`Pulse`) and fill in your email address.
   * **Do NOT click "Publish App"** — keep the Publishing status in **Testing mode**.
   * Under **Test users**, add your personal Google account email (and up to 99 friends/family members who will use your instance).

4. **Create Web Client Credentials:**
   * Go to **APIs & Services → Credentials → Create Credentials → OAuth client ID**.
   * Application type: **Web application**.
   * Under **Authorized redirect URIs**, click **Add URI** and paste exact URI displayed in Pulse:
     ```text
     http://localhost:8080/oauth/callback
     ```
   * Click **Create** and copy your **Client ID** (`*.apps.googleusercontent.com`) and **Client Secret** (`GOCSPX-...`).

5. **Activate Your Pulse Instance:**
   * Paste the copied credentials into Pulse's **Settings Tab** (`http://localhost:8080/settings`) and click **Save & Activate Instance**. You are ready to go!

---

## 🔒 Security & Privacy Architecture

* **No Backend DB:** Pulse never saves metrics to disk.
* **6-Hour Local Browser Cache:** To prevent API rate-limiting during page refreshes, metrics are cached temporarily inside your browser's local storage (`localStorage`). This cache expires automatically after 6 hours and can be purged instantly by clicking **Sign Out** or **Clear Cache**.
* **Encrypted Session Cookie:** Your Google OAuth access token is stored inside an `HttpOnly`, `SameSite=Lax` browser session cookie. Closing your session immediately revokes server access.

---

## License

MIT License. Free and self-hosted forever.
