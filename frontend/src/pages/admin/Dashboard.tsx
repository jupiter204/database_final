import React, { useEffect, useState } from 'react';
import apiClient from '../../services/apiClient';
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/Card';
import { Activity, AlertTriangle, CheckCircle, Clock } from 'lucide-react';
// 引入圖表組件
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip as RechartsTooltip, Legend } from 'recharts';

const Dashboard: React.FC = () => {
  // 儲存統計數據的狀態
  const [stats, setStats] = useState({ total: 0, faulty: 0, pending: 0, faultRate: 0 });
  const [loading, setLoading] = useState(true); // 是否正在讀取中

  // 當組件一出現就去抓數據
  useEffect(() => {
    const fetchStats = async () => {
      try {
        const res = await apiClient.get('/private/equipments');
        const equipments = res.data || [];
        const total = equipments.length;
        const faulty = equipments.filter((eq: any) => eq.status === 'faulty').length;
        const pending = equipments.filter((eq: any) => eq.status === 'pending_maint').length;
        const faultRate = total > 0 ? Math.round((faulty / total) * 100) : 0;
        
        setStats({ total, faulty, pending, faultRate });
      } catch (error) {
        console.error('取得儀表板數據失敗', error);
      } finally {
        setLoading(false);
      }
    };
    fetchStats();
  }, []);

  // 計算正常運作的器材數量
  const normalCount = stats.total - stats.faulty - stats.pending;
  
  // 圖表需要的格式
  const chartData = [
    { name: '正常運作', value: normalCount, color: '#22c55e' },
    { name: '待保養', value: stats.pending, color: '#f59e0b' },
    { name: '故障待修', value: stats.faulty, color: '#ef4444' }
  ];

  // 如果還在載入中，顯示簡單的提示文字
  if (loading) {
    return <div className="p-8 text-muted-foreground">正在載入數據中...</div>;
  }

  return (
    <div className="space-y-6">
      {/* 標題區 */}
      <div>
        <h1 className="text-3xl font-bold tracking-tight">系統儀表板</h1>
        <p className="text-muted-foreground mt-2">即時器材狀態概覽</p>
      </div>

      {/* 四個小卡片：顯示重點數字 */}
      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
        {/* 總數 */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">總器材數</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats.total}</div>
          </CardContent>
        </Card>
        
        {/* 正常數 */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">正常運作</CardTitle>
            <CheckCircle className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-500">{normalCount}</div>
          </CardContent>
        </Card>

        {/* 待保養 */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">待保養</CardTitle>
            <Clock className="h-4 w-4 text-amber-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-amber-500">{stats.pending}</div>
          </CardContent>
        </Card>

        {/* 故障數 */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">故障通報</CardTitle>
            <AlertTriangle className="h-4 w-4 text-destructive" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-destructive">{stats.faulty}</div>
            <p className="text-xs text-muted-foreground mt-1">目前故障率： {stats.faultRate}%</p>
          </CardContent>
        </Card>
      </div>

      {/* 下方的圓餅圖 */}
      <div className="grid gap-6 md:grid-cols-2">
        <Card className="col-span-1">
          <CardHeader>
            <CardTitle>器材狀態分佈圓餅圖</CardTitle>
          </CardHeader>
          <CardContent className="h-[300px]">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={chartData}
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={80}
                  paddingAngle={5}
                  dataKey="value"
                  stroke="none"
                >
                  {chartData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <RechartsTooltip 
                  contentStyle={{ backgroundColor: '#18181b', borderColor: '#27272a', color: '#fafafa' }}
                  itemStyle={{ color: '#fafafa' }}
                />
                <Legend />
              </PieChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      </div>
    </div>
  );
};

export default Dashboard;
