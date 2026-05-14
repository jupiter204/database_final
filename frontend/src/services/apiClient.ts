import axios from 'axios';

// 建立 axios 實體
const apiClient = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
});

// 設定請求攔截器：每次發送請求前，自動帶上 JWT Token
apiClient.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('access_token');
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// 設定回應攔截器：處理 Token 過期 (401) 等通用錯誤
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response && error.response.status === 401) {
      // 若回傳 401 (未授權)，可清除 token 並導向登入頁
      localStorage.removeItem('access_token');
      localStorage.removeItem('refresh_token');
      // 可以考慮強制重新整理或跳轉
      // window.location.href = '/'; 
    }
    return Promise.reject(error);
  }
);

export default apiClient;
