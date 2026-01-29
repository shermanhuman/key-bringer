# Installing Go on WSL (Debian/Ubuntu)

## Quick Install

```bash
# Download latest Go (1.25.6 as of Jan 2026)
wget https://go.dev/dl/go1.25.6.linux-amd64.tar.gz

# Remove old version and extract
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.6.linux-amd64.tar.gz

# Add to PATH (add to ~/.bashrc for persistence)
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify
go version
```

## Alternative: Using apt (may be older version)

```bash
sudo apt update
sudo apt install -y golang-go
go version
```

---

## Also Install ZFS for Testing

ZFS is in the `contrib` repository. Enable it first:

```bash
# Add contrib to sources (Debian/Ubuntu)
sudo sed -i 's/main$/main contrib/' /etc/apt/sources.list

# Or for newer Debian (sources.list.d):
# sudo sed -i 's/main$/main contrib/' /etc/apt/sources.list.d/debian.sources

# Update and install
sudo apt update
sudo apt install -y zfsutils-linux
```

**Note**: ZFS in WSL2 may have kernel module limitations. If `zpool create` fails, test on the actual Debian server instead.
