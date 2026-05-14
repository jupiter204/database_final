# 第一階段：建置 (Build Stage)
FROM node:20-alpine AS builder

# 設定工作目錄
WORKDIR /app

# 複製 package.json 與 package-lock.json (利用 Docker Cache 機制)
COPY package*.json ./

# 安裝依賴套件 (加入 --legacy-peer-deps 解決 vite-plugin-pwa 的相依性衝突)
RUN npm install --legacy-peer-deps

# 複製所有前端原始碼
COPY . .

# 執行 Vite 打包指令 (會輸出到 /app/dist)
RUN npm run build

# 第二階段：Nginx 部署 (Production Stage)
FROM nginx:stable-alpine

# 從第一階段複製打包好的 dist 資料夾，放到 Nginx 預設的靜態檔案伺服器目錄
COPY --from=builder /app/dist /usr/share/nginx/html

# 曝露 80 port
EXPOSE 80

# 啟動 Nginx
CMD ["nginx", "-g", "daemon off;"]
