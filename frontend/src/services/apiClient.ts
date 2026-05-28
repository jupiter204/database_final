import axios from 'axios';

const apiClient = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
});

// 1. 請求攔截器 (Request Interceptor) - 保持不變
apiClient.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('access_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// 2. 💡 新增：回應攔截器 (Response Interceptor) - 專治 Token 過期
apiClient.interceptors.response.use(
  (response) => response, // 正常回應直接放行
  async (error) => {
    const originalRequest = error.config;

    // 當後端回傳 401 Unauthorized，且該請求還沒有重試過
    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true; // 標記此請求已重試，避免無限迴圈
      
      const refreshToken = localStorage.getItem('refresh_token');
      if (!refreshToken) {
        // 連 refresh_token 都沒有，直接登出
        handleForceLogout();
        return Promise.reject(error);
      }

      try {
        // 嘗試向後端發送無痛刷新請求 (注意：此處需使用原生 axios，避免引發原本 apiClient 的攔截迴圈)
        const res = await axios.post('/api/auth/refresh', { refresh_token: refreshToken });
        const newAccessToken = res.data.access_token;

        if (newAccessToken) {
          // 儲存新的 access_token
          localStorage.setItem('access_token', newAccessToken);
          
          // 更新本次失敗請求的 Header，並重新發送
          originalRequest.headers.Authorization = `Bearer ${newAccessToken}`;
          return apiClient(originalRequest);
        }
      } catch (refreshError) {
        // 刷新失敗（代表 refresh_token 也過期或失效了）
        handleForceLogout();
        return Promise.reject(refreshError);
      }
    }

    return Promise.reject(error);
  }
);

// 強制安全登出清空狀態的輔助函式
function handleForceLogout() {
  localStorage.removeItem('access_token');
  localStorage.removeItem('refresh_token');
  if (apiClient.defaults.headers.common['Authorization']) {
    delete apiClient.defaults.headers.common['Authorization'];
  }
  window.location.href = '/login?expired=true'; // 跳回登入頁，並可選提示使用者工作階段過期
}

export default apiClient;