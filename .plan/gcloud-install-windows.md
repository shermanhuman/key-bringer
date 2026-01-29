# Installing Google Cloud CLI on Windows 11

## Quick Install

### 1. Download Installer

Go to: https://cloud.google.com/sdk/docs/install

Or use PowerShell:

```powershell
(New-Object Net.WebClient).DownloadFile("https://dl.google.com/dl/cloudsdk/channels/rapid/GoogleCloudSDKInstaller.exe", "$env:TEMP\GoogleCloudSDKInstaller.exe")
Start-Process "$env:TEMP\GoogleCloudSDKInstaller.exe" -Wait
```

### 2. Run Installer

- Accept defaults
- Check "Run `gcloud init`" at the end

### 3. Initialize

```powershell
gcloud init
```

- Select your Google account
- Choose or create a project

### 4. Verify

```powershell
gcloud --version
gcloud config list
```

---

## Enable Required APIs

```powershell
gcloud services enable `
  secretmanager.googleapis.com `
  run.googleapis.com `
  cloudbuild.googleapis.com
```

---

## Authenticate for Application Default Credentials

```powershell
gcloud auth application-default login
```

This creates credentials at `%APPDATA%\gcloud\application_default_credentials.json` for local development.
