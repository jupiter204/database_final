import React, { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import apiClient from '../../services/apiClient';
import { Button } from '../../components/ui/Button';
import { AlertTriangle, CheckCircle2, Loader2 } from 'lucide-react';

const ReportEquipment: React.FC = () => {
  // 從網址路徑中取得器材 ID (例如 /report/EQU-001)
  const { id } = useParams<{ id: string }>();

  // 狀態管理：儲存器材資料、載入狀態、使用者輸入、送出狀態等
  const [equipment, setEquipment] = useState<any>(null); // 器材資料
  const [loading, setLoading] = useState(true); // 是否載入中
  const [description, setDescription] = useState(''); // 故障描述內容
  const [submitting, setSubmitting] = useState(false); // 是否正在提交表單
  const [submitted, setSubmitted] = useState(false); // 是否提交成功
  const [error, setError] = useState(''); // 錯誤訊息

  // 當頁面第一次載入時，去跟 API 要這台器材的資料
  useEffect(() => {
    const fetchEquipment = async () => {
      if (id) {
        try {
          console.log('正在讀取器材資訊，ID 為:', id);
          const res = await apiClient.get('/public/equipment?asset_code=' + id);
          setEquipment(res.data);
        } catch (err) {
          console.error('無法取得器材狀態', err);
          setEquipment(null);
        } finally {
          setLoading(false);
        }
      }
    };
    fetchEquipment();
  }, [id]);

  // 當使用者按下「提交通報」按鈕時
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault(); // 防止網頁重新整理
    
    // 如果沒有 ID 或者描述是空的，就不處理
    if (!equipment || !description.trim()) {
      alert('請輸入故障描述喔！');
      return;
    }
    
    setSubmitting(true); // 開始顯示「傳送中」
    try {
      await apiClient.post('/public/report', {
        equipment_id: equipment.lid,
        reporter_type: 'public',
        description: description
      });
      setSubmitted(true); // 成功了切換到成功畫面
    } catch (err) {
      setError('報修失敗，可能是已經有通報紀錄，請稍後再試。'); // 失敗了，顯示錯誤訊息
    } finally {
      setSubmitting(false); // 傳送完畢，停止顯示「傳送中」
    }
  };

  // 如果還在讀取資料，就顯示一個轉圈圈
  if (loading) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center">
        <Loader2 className="w-8 h-8 animate-spin text-primary" />
      </div>
    );
  }

  // 如果找不到這台器材，顯示錯誤
  if (!equipment) {
    return (
      <div className="min-h-screen bg-background flex flex-col items-center justify-center p-6 text-center">
        <AlertTriangle className="w-16 h-16 text-destructive mb-4" />
        <h1 className="text-2xl font-bold mb-2">找不到器材</h1>
        <p className="text-muted-foreground">請確認您掃描的 QR Code 是否正確。</p>
      </div>
    );
  }

  // 如果這台器材已經被報修了，或者剛剛報修成功，就顯示「感謝」畫面
  if (equipment.has_active_report || submitted) {
    return (
      <div className="min-h-screen bg-background flex flex-col items-center justify-center p-6 text-center">
        <CheckCircle2 className="w-16 h-16 text-green-500 mb-4" />
        <h1 className="text-2xl font-bold mb-2">已收到通報</h1>
        <p className="text-muted-foreground text-lg">
          {submitted ? '感謝您的協助，我們會盡快處理！' : '此器材已通報故障，維修人員處理中，感謝您的提醒！'}
        </p>
      </div>
    );
  }

  // 主要的報修表單頁面
  return (
    <div className="min-h-screen bg-background p-6 flex flex-col max-w-md mx-auto">
      <div className="flex-1">
        <h1 className="text-2xl font-bold text-foreground mb-6">回報器材故障</h1>
        
        {/* 顯示器材的基本資訊 */}
        <div className="bg-card border border-border p-4 rounded-xl mb-6 shadow-sm">
          <h2 className="text-lg font-semibold mb-1 text-primary">{equipment.name}</h2>
          <p className="text-sm text-muted-foreground mb-1">編號: {equipment.asset_code}</p>
          <p className="text-sm text-muted-foreground">位置: {equipment.location}</p>
        </div>

        {/* 使用者輸入表單 */}
        <form onSubmit={handleSubmit} className="space-y-4 flex flex-col">
          <div>
            <label className="block text-sm font-medium mb-2 text-foreground">故障狀況描述</label>
            <textarea
              required
              rows={4}
              className="w-full bg-input border border-border rounded-lg p-3 text-foreground focus:ring-2 focus:ring-ring focus:outline-none resize-none transition-shadow"
              placeholder="請簡述您遇到的問題 (例如：螢幕無畫面、異音...)"
              value={description}
              onChange={(e) => setDescription(e.target.value)} // 把輸入的內容存起來
            />
          </div>
          
          {/* 如果有錯誤就顯示紅字 */}
          {error && <p className="text-destructive text-sm">{error}</p>}
          
          {/* 送出按鈕 */}
          <Button 
            type="submit" 
            size="lg" 
            className="w-full text-lg mt-4 h-12"
            disabled={submitting || !description.trim()} // 如果正在送出或沒寫內容，就把按鈕鎖起來
          >
            {submitting ? <Loader2 className="w-5 h-5 animate-spin mr-2" /> : null}
            提交通報
          </Button>
        </form>
      </div>
    </div>
  );
};

export default ReportEquipment;
