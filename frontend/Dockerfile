# --- 第一階段：使用 Bun 進行編譯建置 ---
FROM oven/bun:1.3.14-alpine AS builder
WORKDIR /app

# 複製環境設定與套件清單
# 將原本的 COPY package.json bun.lockb ./ 改為：
COPY package.json bun.lockb* ./

# 使用 Bun 高速安裝套件
RUN bun install --frozen-lockfile

# 複製完整原始碼並進行打包
COPY . .
RUN bun run build

# --- 第二階段：使用 Nginx 部署靜態檔案 ---
FROM nginx:alpine
# 根據你的 nginx.conf 設定複製打包後的檔案（Vite 預設為 dist）
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]