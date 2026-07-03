# Pulse 📊 (Stateless Edition)

Pulse is a sleek, **stateless health dashboard** written in pure Go that integrates directly with the **Google Health API** (the unified replacement for the Fitbit Web API). It allows users to track, analyze, and visualize their health data over time—including sleep stages, Heart Rate Variability (HRV), Resting Heart Rate (RHR), and daily activity/energy metrics.

Designed for **zero-maintenance cloud hosting** or effortless local execution, Pulse operates **100% in-memory and on-the-fly**. No databases (SQLite/Postgres), no disk volumes, no cron background workers, and zero data privacy liabilities!

![Pulse Dashboard](https://images.unsplash.com/photo-1576091160550-2173dba999ef?auto=format&fit=crop&q=80&w=800) *(Stateless health dashboard overview)*

---

## ✨ Why Stateless?

- **Zero Data Privacy Liability:** User health metrics are fetched live from Google APIs when they view their dashboard and vanish from server memory when their session closes. **Nothing is ever stored to disk or a database.**
- **Effortless Deployment:** No database migrations, backups, or storage volume permissions. Deploy instantly to serverless containers like **Google Cloud Run**, **Fly.io**, **AWS App Runner**, or standard Docker.
- **Concurrent Live Fetching:** Utilizes Go goroutines to query Sleep, Heart Rate, HRV, and Activity metrics simultaneously for super-fast dashboard rendering.
- **Secure Cookie Sessions:** OAuth 2.0 tokens are encrypted and stored in secure, HTTP-only browser session cookies.

---

## 🛠️ Google Cloud API Setup (Required for App Owner)

As the host/deployer of Pulse, you only configure Google Cloud OAuth credentials **once** via environment variables. Your users never need to touch the GCP console or enter Client IDs/Secrets!

### Step 1: Create a Google Cloud Project
1. Open the [Google Cloud Console](https://console.cloud.google.com/).
2. Create a new project (e.g., `Pulse Health SaaS`).

### Step 2: Enable the Google Health API
1. Navigate to **APIs & Services** > **Library**.
2. Search for **Google Health API** and click **Enable**.

### Step 3: Configure OAuth Consent Screen
1. Go to **APIs & Services** > **OAuth consent screen**.
2. Select **External** (or **Internal** for Google Workspace organizations).
3. Add the following required scopes:
   - `https://www.googleapis.com/auth/googlehealth.sleep.readonly`
   - `https://www.googleapis.com/auth/googlehealth.health_metrics_and_measurements.readonly`
   - `https://www.googleapis.com/auth/googlehealth.activity_and_fitness.readonly`
   - `https://www.googleapis.com/auth/googlehealth.profile.readonly`
   - `https://www.googleapis.com/auth/userinfo.email`
   - `https://www.googleapis.com/auth/userinfo.profile`

### Step 4: Create Credentials
1. Go to **APIs & Services** > **Credentials**.
2. Click **Create Credentials** > **OAuth client ID** > **Web application**.
3. Under **Authorized redirect URIs**, add your domain or local development URL:
   - `http://localhost:8080/oauth/callback` *(Local testing)*
   - `https://your-app-domain.com/oauth/callback` *(Production hosted)*
4. Copy your **Client ID** and **Client Secret**.

---

## 🚀 Getting Started with Docker

### 1. Configure Environment Variables
You can pass your OAuth credentials directly via terminal or using `docker-compose.yml`:

```yaml
version: '3.8'

services:
  pulse:
    image: ghcr.io/jleagle/pulse:latest # Or build locally using 'build: .'
    container_name: pulse-stateless
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - CLIENT_ID=your-google-client-id.apps.googleusercontent.com
      - CLIENT_SECRET=your-google-client-secret
      - REDIRECT_URL=http://localhost:8080/oauth/callback
    restart: unless-stopped
```

### 2. Run the Container
```bash
# Export variables and launch
export CLIENT_ID="your-client-id"
export CLIENT_SECRET="your-client-secret"
docker-compose up -d
```

### 3. Use the App
1. Open your browser and go to `http://localhost:8080`.
2. Click **Connect Google Health**.
3. Grant authorization via Google's consent screen.
4. Watch your live sleep, heart rate, and activity charts render immediately!

---

## ⚙️ Environment Variables

| Variable | Description | Default |
| :--- | :--- | :--- |
| `PORT` | The HTTP port the container serves on. | `8080` |
| `CLIENT_ID` | Your Google Cloud OAuth Client ID. | *(Required)* |
| `CLIENT_SECRET` | Your Google Cloud OAuth Client Secret. | *(Required)* |
| `REDIRECT_URL` | The OAuth callback URL registered in GCP. | `http://localhost:8080/oauth/callback` |
