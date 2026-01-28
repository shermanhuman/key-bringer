# Installing Go on Windows 11

## Quick Install (Recommended)

### 1. Download the Installer

1. Go to [go.dev/dl/](https://go.dev/dl/)
2. Download the Windows MSI installer (e.g., `go1.25.6.windows-amd64.msi`)

### 2. Run the Installer

1. Double-click the downloaded `.msi` file
2. Click **Next** on the welcome screen
3. Accept the license agreement
4. Keep the default install path: `C:\Program Files\Go`
5. Click **Install**
6. Click **Finish**

### 3. Restart Your Terminal

Close and reopen PowerShell or your terminal. The installer automatically adds Go to your PATH.

### 4. Verify Installation

```powershell
go version
# Expected: go version go1.25.6 windows/amd64
```

---

## VS Code Setup (Optional but Recommended)

1. Install the **Go extension** by the Go Team at Google
2. Open a `.go` file
3. VS Code will prompt to install Go tools — click **Install All**

---

## Alternative: Chocolatey

If you use Chocolatey package manager:

```powershell
choco install golang
```

---

## Troubleshooting

### `go` command not found after install

The PATH wasn't updated. Either:

1. Restart your computer, OR
2. Manually add to PATH:
   - Open **System Properties** → **Environment Variables**
   - Under **System variables**, find `Path`
   - Add: `C:\Program Files\Go\bin`

### Check your Go environment

```powershell
go env
```

Key variables:

- `GOROOT`: Where Go is installed (e.g., `C:\Program Files\Go`)
- `GOPATH`: Your workspace (default: `C:\Users\<you>\go`)
