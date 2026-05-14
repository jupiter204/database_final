export interface User {
  id: string;
  username: string;
  name: string;
  role: 'admin' | 'staff';
}

export interface Equipment {
  id: string;
  asset_code: string;
  name: string;
  category: string;
  last_maint_date: string;
  maint_interval: number;
  next_maint_date: string;
  status: 'normal' | 'pending_maint' | 'repairing' | 'faulty';
  location: string;
  has_active_report?: boolean;
}

export interface MaintenanceRecord {
  id: number;
  equipment_id: string;
  reporter_id?: string;
  reporter_type: 'public' | 'staff';
  description: string;
  photo_url?: string;
  is_resolved: boolean;
  resolve_note?: string;
  created_at: string;
}
