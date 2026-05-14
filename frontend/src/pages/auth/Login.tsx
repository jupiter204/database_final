import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '../../components/ui/Button';
import { Activity } from 'lucide-react';
import apiClient from '../../services/apiClient';

const Login: React.FC = () => {
  // 用來切換網址的工具
  const navigate = useNavigate();

  // 用來儲存使用者輸入的內容 (useState)
  const [username, setUsername] = useState(''); // 帳號
  const [password, setPassword] = useState(''); // 密碼

  // 當使用者按下「登入系統」按鈕時會觸發這個函式
  const handleLogin = async (e: React.FormEvent) => {
    // 防止網頁因為送出表單而重新整理
    e.preventDefault();

    if (username !== '' && password !== '') {
      try {
        console.log('正在嘗試登入...');
        
        // 向 Go 後端發送真實的登入請求
        const response = await apiClient.post('/auth/login', {
          username: username,
          password: password
        });

        // 將取得的 Token 存進瀏覽器的 localStorage
        localStorage.setItem('access_token', response.data.access_token);
        if (response.data.refresh_token) {
          localStorage.setItem('refresh_token', response.data.refresh_token);
        }

        // 成功後跳轉到管理後台首頁
        navigate('/admin');
      } catch (err: any) {
        console.error('登入失敗:', err);
        alert('登入失敗，請確認帳號密碼是否正確！');
      }
    } else {
      alert('請輸入帳號和密碼喔！');
    }
  };

  return (
    <div className="min-h-screen bg-background flex flex-col items-center justify-center p-4">
      <div className="w-full max-w-sm">
        
        {/* 標題與標誌區塊 */}
        <div className="flex flex-col items-center mb-8">
          <div className="w-16 h-16 bg-primary/10 rounded-full flex items-center justify-center mb-4">
            <Activity className="w-8 h-8 text-primary" />
          </div>
          <h1 className="text-3xl font-bold tracking-tight">GETS</h1>
          <p className="text-muted-foreground mt-2">運動健身器材維護系統</p>
        </div>

        {/* 登入輸入框區塊 */}
        <div className="bg-card border border-border rounded-xl shadow-lg p-6">
          <form onSubmit={handleLogin} className="space-y-4">
            
            {/* 帳號欄位 */}
            <div>
              <label className="block text-sm font-medium mb-1.5">帳號</label>
              <input
                type="text"
                required
                className="w-full bg-input border border-border rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-ring"
                value={username}
                onChange={(e) => setUsername(e.target.value)} // 把輸入的東西存進 username 變數
                placeholder="請輸入帳號"
              />
            </div>

            {/* 密碼欄位 */}
            <div>
              <label className="block text-sm font-medium mb-1.5">密碼</label>
              <input
                type="password"
                required
                className="w-full bg-input border border-border rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-ring"
                value={password}
                onChange={(e) => setPassword(e.target.value)} // 把輸入的東西存進 password 變數
                placeholder="請輸入密碼"
              />
            </div>

            {/* 送出按鈕 */}
            <Button type="submit" className="w-full mt-4">
              登入系統
            </Button>

          </form>
        </div>
      </div>
    </div>
  );
};

export default Login;
