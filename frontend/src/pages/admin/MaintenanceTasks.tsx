import { useEffect, useState } from 'react';
import apiClient from '../../services/apiClient';
import { Card, CardContent } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Wrench, Check, AlertTriangle } from 'lucide-react';

const MaintenanceTasks: React.FC = () => {
  // 儲存待處理任務清單的狀態
  const [tasks, setTasks] = useState<any[]>([]);
  const [loading, setLoading] = useState(true); // 是否載入中

  // 當頁面打開時，執行抓取任務的函式
  useEffect(() => {
    fetchTasks();
  }, []);

  // 向api要求"待處理維修任務"的函式
  const fetchTasks = async () => {
    setLoading(true); // 開始轉圈圈
    try {
      const res = await apiClient.get('/private/maintenance-records');
      // 過濾出尚未解決的任務 (以防後端回傳所有任務)
      const pendingTasks = (res.data || []).filter((task: any) => !task.is_resolved);
      setTasks(pendingTasks);
    } catch (error) {
      console.error('無法取得維修任務清單:', error);
    } finally {
      setLoading(false); // 停止轉圈圈
    }
  };

  // 當點擊「標記為已解決」按鈕時
  const handleResolve = async (lid: string) => {
    try {
      console.log('正在將任務 ID', lid, '標記為已解決...');
      await apiClient.patch('/private/maintenance-records/resolve', {
        lid,
        resolve_note: '已完成修復'
      });
      alert('任務已成功解決！');
      fetchTasks(); // 重新整理清單，把解決掉的任務移出畫面
    } catch (error) {
      console.error('解決任務失敗:', error);
      alert('標記失敗，請稍後再試！');
    }
  };

  // 載入畫面
  if (loading) return <div className="text-muted-foreground p-8">正在努力載入任務清單...</div>;

  return (
    <div className="space-y-6">
        <h1 className="text-3xl font-bold tracking-tight flex items-center gap-3">
          <Wrench className="w-8 h-8 text-primary" />
          待處理的維修任務
        </h1>

      {/* 如果沒有任務時顯示的畫面 */}
      {tasks.length === 0 ? (
        <Card className="border-dashed bg-secondary/50">
          <CardContent className="flex flex-col items-center justify-center p-12 text-center">
            <Check className="w-12 h-12 text-green-500 mb-4" />
            <p className="text-lg font-medium">太棒了！目前沒有待處理的任務</p>
            <p className="text-muted-foreground">所有的健身器材目前都非常健康！</p>
          </CardContent>
        </Card>
      ) : (
        /* 有任務時，循環顯示每一個任務卡片 */
        <div className="grid gap-4">
          {tasks.map(task => (
            <Card key={task.lid || task.id} className="border-destructive/30 relative overflow-hidden">
              {/* 卡片左側的紅色條，表示緊急或重要 */}
              <div className="absolute top-0 left-0 w-1 h-full bg-destructive"></div>
              <CardContent className="p-6">
                <div className="flex flex-col md:flex-row justify-between gap-6">
                  <div className="flex-1 space-y-2">
                    <div className="flex items-center gap-2">
                      <AlertTriangle className="w-5 h-5 text-destructive" />
                      <h3 className="font-semibold text-lg">故障通報單 - 器材: {task.equipment_name || task.asset_code}</h3>
                    </div>
                    {/* 故障描述框 */}
                    <p className="text-muted-foreground bg-secondary/50 p-3 rounded-md">
                      問題描述： {task.description}
                    </p>
                    {/* 任務詳細時間與來源 */}
                    <div className="text-sm text-muted-foreground flex gap-4">
                      <span>通報時間: {new Date(task.created_at).toLocaleString()}</span>
                      <span>通報人身分: {task.reporter_type === 'public' ? '一般民眾' : '健身房員工'}</span>
                    </div>
                  </div>
                  {/* 按鈕區 */}
                  <div className="flex items-end">
                    <Button 
                      onClick={() => handleResolve(task.lid || task.id)}
                      className="bg-green-600 hover:bg-green-700 text-white flex items-center gap-2"
                    >
                      <Check className="w-4 h-4" /> 點我解決此任務
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
};

export default MaintenanceTasks;
