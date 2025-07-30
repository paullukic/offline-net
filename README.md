# 📚 Offline-Net — Your Personal Offline Knowledge Server

> **Take the internet with you** — Serve Wikipedia, educational resources, and your own websites **without an internet connection**.

Offline-Net is a lightweight Go-powered server for hosting **ZIM files** (compressed offline web archives, e.g., Wikipedia, Wiktionary, StackExchange) and **custom offline websites**.  
It gives you a clean dashboard to browse all your offline content — perfect for remote areas, ships, classrooms, or just to have the internet in your pocket.

---

## ✨ Features

- 📦 **Serve ZIM files instantly** — Powered by [Bornholm/go-zim](https://pkg.go.dev/github.com/Bornholm/go-zim) for lightning-fast reading.
- 🌐 **Custom offline websites** — Drop static HTML into a folder and serve it instantly.
- 🖥 **Beautiful dashboard** — All content neatly listed at http://localhost:8000/.
- 🔌 **Cross-platform builds** — Windows, Linux, macOS, Raspberry Pi.
- 🚀 **Zero external dependencies at runtime** — Just drop files in and run.
- 📡 **Portable knowledge** — Use it in classrooms, remote villages, on boats, or during travel.

---

## 📸 Screenshot

Offline-Net Dashboard
<img width="1049" height="1121" alt="image" src="https://github.com/user-attachments/assets/727cb4c9-9ca4-4f2b-b230-cd5a4d2aa4be" />
---

## 📂 Project Structure
```graphql
offline-net/
├── build.sh             # Cross-platform build script
├── main.go               # Main server entry point
├── zim.go                # ZIM file handling logic
├── templates/
│   └── library.html      # Dashboard template
├── static/               
├── zim-content/          # Place your .zim files here
└── web-content/          # Custom static websites go here
```

---

## 🚀 Getting Started

### 1️⃣ Install Go
Make sure you have [Go 1.20+](https://go.dev/dl/) installed.

### 2️⃣ Clone the repository
```bash
git clone https://github.com/yourusername/offline-net.git
cd offline-net
```

3️⃣ Add your content
ZIM files → put them in ./zim-content

```
Custom sites → put them in ./web-content as folders containing index.html
```

4️⃣ Run the server
```bash
go run main.go
```

Then open:
http://localhost:8000/

---
## 🔨 Building for Your System

Use the included build.sh script:

```bash
chmod +x build.sh
./build.sh -system <windows|linux|macos|raspberry> -bits <32|64>
```
Examples:

```bash
./build.sh -system windows -bits 64    # Windows 64-bit
./build.sh -system linux -bits 32      # Linux 32-bit
./build.sh -system macos -bits 64      # macOS 64-bit
./build.sh -system raspberry -bits 32  # Raspberry Pi 32-bit
```

## The compiled binary will appear in the project folder.

### 🌍 Usage Idea — Knowledge Anywhere
### 📚 Bring Wikipedia into classrooms without internet
### 🏝 Use on boats or remote expeditions
### 🏫 Equip offline schools in developing countries
### ✈️ Have the internet in your pocket during travel
### With Offline-Net, you’ll have all the knowledge on the go, even without the internet.


###🙏 Credits
Kiwix — The ZIM file ecosystem
Bornholm/go-zim — Go ZIM reader

📜 License
MIT License — feel free to use, modify, and share.
