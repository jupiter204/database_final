import React, { useEffect, useState } from 'react';
import apiClient from '../../services/apiClient';
import { Card, CardContent } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Plus, Search, MapPin, Tag, X } from 'lucide-react';

const EquipmentList: React.FC = () => {
  const [equipments, setEquipments] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  
  // 新增設備用的狀態
  const [showAddModal, setShowAddModal] = useState(false);
  const [formData, setFormData] = useState({
    asset_code: '',
    name: '',
    category: '',
    location: '',
    maint_interval: 30,
  });

  // 編輯設備用的狀態
  const [showEditModal, setShowEditModal] = useState(false);
  const [editFormData, setEditFormData] = useState({
    lid: '',
    asset_code: '',
    name: '',
    category: '',
    location: '',
    maint_interval: 30,
  });

  // QR Code 視窗狀態
  const [showQrModal, setShowQrModal] = useState(false);
  const [qrData, setQrData] = useState({ name: '', code: '' });

  const [isSubmitting, setIsSubmitting] = useState(false);

  // 取得真實的 API 資料
  const fetchEquipments = async () => {
    try {
      const res = await apiClient.get('/private/equipments');
      setEquipments(res.data || []);
    } catch (err) {
      console.error('取得設備列表失敗:', err);
      alert('無法取得設備列表，請確認是否已經登入！');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchEquipments();
  }, []);

  // 處理新增表單變更
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;
    setFormData(prev => ({ ...prev, [name]: value }));
  };

  // 處理編輯表單變更
  const handleEditInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;
    setEditFormData(prev => ({ ...prev, [name]: value }));
  };

  // 打開編輯視窗
  const handleEditClick = (eq: any) => {
    setEditFormData({
      lid: eq.lid || eq.id,
      asset_code: eq.asset_code || '',
      name: eq.name || '',
      category: eq.category || '',
      location: eq.location || '',
      maint_interval: eq.maint_interval || 30,
    });
    setShowEditModal(true);
  };

  // 送出新增請求
  const handleAddSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    try {
      await apiClient.post('/private/equipment', {
        asset_code: formData.asset_code,
        name: formData.name,
        category: formData.category,
        location: formData.location,
        maint_interval: Number(formData.maint_interval),
      });
      alert('新增成功！');
      setShowAddModal(false);
      setFormData({ asset_code: '', name: '', category: '', location: '', maint_interval: 30 });
      fetchEquipments();
    } catch (err: any) {
      console.error('新增失敗:', err);
      alert(err.response?.data?.error || '新增失敗，請確認編號是否重複或格式錯誤！');
    } finally {
      setIsSubmitting(false);
    }
  };

  // 送出編輯請求
  const handleEditSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    try {
      await apiClient.patch('/private/equipment', {
        lid: editFormData.lid,
        asset_code: editFormData.asset_code,
        name: editFormData.name,
        category: editFormData.category,
        location: editFormData.location,
        maint_interval: Number(editFormData.maint_interval),
      });
      alert('修改成功！');
      setShowEditModal(false);
      fetchEquipments();
    } catch (err: any) {
      console.error('修改失敗:', err);
      alert(err.response?.data?.error || '修改失敗，請確認資料是否正確！');
    } finally {
      setIsSubmitting(false);
    }
  };

  // 刪除設備
  const handleDelete = async (lid: string) => {
    if (!window.confirm('確定要刪除這個設備嗎？這將會連帶刪除它的所有維修紀錄！')) return;
    try {
      await apiClient.delete('/private/equipment', { data: { lid } });
      alert('刪除成功！');
      fetchEquipments();
    } catch (err: any) {
      console.error('刪除失敗:', err);
      alert('刪除失敗！');
    }
  };

  // 打開 QR Code 視窗
  const handleQrClick = (eq: any) => {
    setQrData({ name: eq.name, code: eq.asset_code });
    setShowQrModal(true);
  };

  const getStatusBadge = (status: string) => {
    if (status === 'normal') {
      return <span className="px-2 py-1 bg-green-500/20 text-green-500 rounded-full text-xs whitespace-nowrap">狀態正常</span>;
    } else if (status === 'pending_maint') {
      return <span className="px-2 py-1 bg-amber-500/20 text-amber-500 rounded-full text-xs whitespace-nowrap">待保養</span>;
    } else if (status === 'faulty') {
      return <span className="px-2 py-1 bg-destructive/20 text-destructive rounded-full text-xs whitespace-nowrap">故障待修</span>;
    } else {
      return <span className="px-2 py-1 bg-secondary text-secondary-foreground rounded-full text-xs whitespace-nowrap">未知狀態</span>;
    }
  };

  if (loading) return <div className="text-muted-foreground p-8">正在讀取器材資料，請稍候...</div>;

  return (
    <div className="space-y-6 relative">
      <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">器材管理列表</h1>
          <p className="text-muted-foreground mt-2">在這裡檢視所有健身器材的狀態與位置</p>
        </div>
        <Button className="flex items-center gap-2" onClick={() => setShowAddModal(true)}>
          <Plus className="w-4 h-4" /> 新增器材
        </Button>
      </div>

      <div className="flex gap-4 items-center">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <input 
            type="text" 
            placeholder="搜尋器材..." 
            className="w-full bg-card border border-border rounded-lg pl-10 pr-4 py-2 focus:outline-none focus:ring-2 focus:ring-primary"
          />
        </div>
      </div>

      <div className="grid gap-4">
        {equipments.length === 0 ? (
           <div className="text-center p-8 text-muted-foreground border rounded-lg border-dashed">
             目前還沒有任何設備資料。
           </div>
        ) : equipments.map(eq => (
          <Card key={eq.lid || eq.id}>
            <CardContent className="p-4 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
              <div className="flex-1 space-y-2">
                <div className="flex items-center gap-3">
                  <span className="font-bold text-lg">{eq.name}</span>
                  {getStatusBadge(eq.status)}
                </div>
                <div className="flex flex-wrap gap-4 text-sm text-muted-foreground">
                  <div className="flex items-center gap-1">
                    <Tag className="w-3 h-3" /> 編號: {eq.asset_code}
                  </div>
                  <div className="flex items-center gap-1">
                    <MapPin className="w-3 h-3" /> 位置: {eq.location || '未指定'}
                  </div>
                  <div>分類: {eq.category || '未分類'}</div>
                  <div>上次保養: {eq.last_maint_date || '無紀錄'}</div>
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-2 mt-2 sm:mt-0">
                <Button variant="outline" size="sm" onClick={() => handleEditClick(eq)}>編輯資料</Button>
                <Button variant="secondary" size="sm" onClick={() => handleQrClick(eq)}>QR Code</Button>
                <Button variant="destructive" size="sm" onClick={() => handleDelete(eq.lid || eq.id)}>刪除</Button>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* 新增設備的彈出視窗 (Modal) */}
      {showAddModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-card w-full max-w-md rounded-xl shadow-xl overflow-hidden border border-border">
            <div className="flex justify-between items-center p-4 border-b border-border">
              <h2 className="text-xl font-bold">新增健身器材</h2>
              <button onClick={() => setShowAddModal(false)} className="text-muted-foreground hover:text-foreground">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleAddSubmit} className="p-4 space-y-4">
              <div>
                <label className="block text-sm font-medium mb-1.5">資產編號 (必填)</label>
                <input required type="text" name="asset_code" value={formData.asset_code} onChange={handleInputChange} className="w-full bg-input border border-border rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary" placeholder="例如: EQ-003" />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1.5">器材名稱 (必填)</label>
                <input required type="text" name="name" value={formData.name} onChange={handleInputChange} className="w-full bg-input border border-border rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary" placeholder="例如: 飛輪車 B" />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1.5">分類</label>
                <input type="text" name="category" value={formData.category} onChange={handleInputChange} className="w-full bg-input border border-border rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary" placeholder="例如: 心肺器材" />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1.5">放置位置</label>
                <input type="text" name="location" value={formData.location} onChange={handleInputChange} className="w-full bg-input border border-border rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary" placeholder="例如: 1樓有氧區" />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1.5">保養週期 (天數)</label>
                <input required type="number" name="maint_interval" min="1" value={formData.maint_interval} onChange={handleInputChange} className="w-full bg-input border border-border rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary" />
              </div>
              <div className="pt-4 flex justify-end gap-2">
                <Button type="button" variant="outline" onClick={() => setShowAddModal(false)}>取消</Button>
                <Button type="submit" disabled={isSubmitting}>{isSubmitting ? '新增中...' : '確認新增'}</Button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* 編輯設備的彈出視窗 (Modal) */}
      {showEditModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-card w-full max-w-md rounded-xl shadow-xl overflow-hidden border border-border">
            <div className="flex justify-between items-center p-4 border-b border-border">
              <h2 className="text-xl font-bold">編輯健身器材</h2>
              <button onClick={() => setShowEditModal(false)} className="text-muted-foreground hover:text-foreground">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleEditSubmit} className="p-4 space-y-4">
              <div>
                <label className="block text-sm font-medium mb-1.5">資產編號 (必填)</label>
                <input required type="text" name="asset_code" value={editFormData.asset_code} onChange={handleEditInputChange} className="w-full bg-input border border-border rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary" />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1.5">器材名稱 (必填)</label>
                <input required type="text" name="name" value={editFormData.name} onChange={handleEditInputChange} className="w-full bg-input border border-border rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary" />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1.5">分類</label>
                <input type="text" name="category" value={editFormData.category} onChange={handleEditInputChange} className="w-full bg-input border border-border rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary" />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1.5">放置位置</label>
                <input type="text" name="location" value={editFormData.location} onChange={handleEditInputChange} className="w-full bg-input border border-border rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary" />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1.5">保養週期 (天數)</label>
                <input required type="number" name="maint_interval" min="1" value={editFormData.maint_interval} onChange={handleEditInputChange} className="w-full bg-input border border-border rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary" />
              </div>
              <div className="pt-4 flex justify-end gap-2">
                <Button type="button" variant="outline" onClick={() => setShowEditModal(false)}>取消</Button>
                <Button type="submit" disabled={isSubmitting}>{isSubmitting ? '修改中...' : '確認修改'}</Button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* QR Code 顯示視窗 (Modal) */}
      {showQrModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-card w-full max-w-sm rounded-xl shadow-xl overflow-hidden border border-border">
            <div className="flex justify-between items-center p-4 border-b border-border">
              <h2 className="text-xl font-bold">專屬 QR Code</h2>
              <button onClick={() => setShowQrModal(false)} className="text-muted-foreground hover:text-foreground">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="p-6 flex flex-col items-center space-y-4">
              <p className="text-center text-muted-foreground text-sm">
                請將此 QR Code 印出並貼在「{qrData.name}」上，民眾掃描後即可直接進入報修頁面。
              </p>
              
              <div className="bg-white p-4 rounded-lg shadow-inner">
                {/* 透過免費 API 直接生成 QR Code，URL 帶入當前網址的主機名稱，加上 /report/:asset_code */}
                <img 
                  src={`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(window.location.origin + '/report/' + qrData.code)}`} 
                  alt="Equipment QR Code" 
                  className="w-48 h-48"
                />
              </div>
              
              <div className="text-center font-mono bg-secondary/50 px-4 py-2 rounded w-full">
                {qrData.code}
              </div>
              
              <div className="pt-2 w-full">
                <Button className="w-full" onClick={() => window.print()}>列印 QR Code</Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default EquipmentList;
