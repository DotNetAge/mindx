import { useState, useEffect } from 'react';
import './styles/Cron.css';
import { useTranslation } from '../i18n';

interface Job {
  id: string;
  name: string;
  cron: string;
  message: string;
  command: string;
  enabled: boolean;
  created_at: string;
  last_run?: string;
  last_status: 'pending' | 'running' | 'success' | 'error';
  last_error?: string;
}

interface JobsResponse {
  jobs: Job[];
}

export default function Cron() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showDialog, setShowDialog] = useState(false);
  const [editingJob, setEditingJob] = useState<Job | null>(null);
  const [formData, setFormData] = useState<Partial<Job>>({
    name: '',
    cron: '',
    message: '',
    command: '',
    enabled: true,
  });
  const [actionLoading, setActionLoading] = useState(false);
  const [actionMessage, setActionMessage] = useState('');
  const { t } = useTranslation();

  useEffect(() => {
    fetchJobs();
  }, []);

  const fetchJobs = async () => {
    try {
      setLoading(true);
      const response = await fetch('/api/cron/jobs');
      if (!response.ok) {
        throw new Error('Failed to fetch jobs');
      }
      const data: JobsResponse = await response.json();
      setJobs(data.jobs || []);
    } catch (error) {
      console.error('Failed to fetch jobs:', error);
      setError(t('cron.loadFailed'));
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = () => {
    setEditingJob(null);
    setFormData({
      name: '',
      cron: '',
      message: '',
      command: '',
      enabled: true,
    });
    setShowDialog(true);
  };

  const handleEdit = (job: Job) => {
    setEditingJob(job);
    setFormData({ ...job });
    setShowDialog(true);
  };

  const handleDelete = async (job: Job) => {
    if (!confirm(t('cron.confirmDelete', { name: job.name }))) {
      return;
    }

    try {
      setActionLoading(true);
      setActionMessage(t('cron.deleting'));

      const response = await fetch(`/api/cron/jobs/${job.id}`, {
        method: 'DELETE',
      });
      if (!response.ok) {
        throw new Error('Failed to delete job');
      }

      alert(t('cron.deleteSuccess'));
      fetchJobs();
    } catch (error) {
      console.error('Failed to delete job:', error);
      alert(t('cron.deleteFailed'));
    } finally {
      setActionLoading(false);
      setActionMessage('');
    }
  };

  const handleTogglePause = async (job: Job) => {
    const action = job.enabled ? 'pause' : 'resume';
    try {
      setActionLoading(true);
      setActionMessage(action === 'pause' ? t('cron.pauseAction') : t('cron.resumeAction'));

      const response = await fetch(`/api/cron/jobs/${job.id}/${action}`, {
        method: 'POST',
      });
      if (!response.ok) {
        throw new Error(`Failed to ${action} job`);
      }

      fetchJobs();
    } catch (error) {
      console.error(`Failed to ${action} job:`, error);
      alert(action === 'pause' ? t('cron.pauseFailed') : t('cron.resumeFailed'));
    } finally {
      setActionLoading(false);
      setActionMessage('');
    }
  };

  const handleSave = async () => {
    try {
      setActionLoading(true);
      setActionMessage(t('cron.saveAction'));

      const isEdit = editingJob !== null;
      const url = isEdit ? `/api/cron/jobs/${editingJob.id}` : '/api/cron/jobs';
      const method = isEdit ? 'PUT' : 'POST';

      const response = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(formData),
      });
      if (!response.ok) {
        throw new Error('Failed to save job');
      }

      alert(t('cron.saveSuccess'));
      setShowDialog(false);
      fetchJobs();
    } catch (error) {
      console.error('Failed to save job:', error);
      alert(t('cron.saveFailed'));
    } finally {
      setActionLoading(false);
      setActionMessage('');
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'success': return '✅';
      case 'running': return '🔄';
      case 'error': return '❌';
      default: return '⏳';
    }
  };

  const getStatusText = (status: string) => {
    switch (status) {
      case 'success': return t('cron.statusSuccess');
      case 'running': return t('cron.statusRunning');
      case 'error': return t('cron.statusError');
      default: return t('cron.statusPending');
    }
  };

  if (loading) {
    return (
      <div className="cron-container">
        <div className="loading">{t('common.loading')}</div>
      </div>
    );
  }

  return (
    <div className="cron-container">
      <div className="cron-header">
        <h1>{t('cron.title')}</h1>
        <div className="header-actions">
          <button className="action-btn secondary" onClick={fetchJobs}>
            {t('cron.refresh')}
          </button>
          <button className="action-btn primary" onClick={handleAdd}>
            {t('cron.addTask')}
          </button>
        </div>
      </div>

      {error && (
        <div className="error-message">{error}</div>
      )}

      <div className="cron-content">
        {jobs.length === 0 ? (
          <div className="empty-state">
            <p>{t('cron.noTasks')}</p>
            <small>{t('cron.noTasksHint')}</small>
          </div>
        ) : (
          <div className="jobs-list">
            {jobs.map((job) => (
              <div key={job.id} className="job-card">
                <div className="job-header">
                  <div className="job-title">
                    <h3>{job.name}</h3>
                    <span className={`job-enabled ${job.enabled ? 'enabled' : 'disabled'}`}>
                      {job.enabled ? t('cron.enabled') : t('cron.paused')}
                    </span>
                  </div>
                  <div className="job-status">
                    <span className="status-badge">
                      {getStatusIcon(job.last_status)} {getStatusText(job.last_status)}
                    </span>
                  </div>
                </div>

                <div className="job-details">
                  <div className="detail-item">
                    <label>{t('cron.cronExpr')}</label>
                    <span>{job.cron}</span>
                  </div>
                  {job.message && (
                    <div className="detail-item">
                      <label>{t('cron.message')}</label>
                      <span>{job.message}</span>
                    </div>
                  )}
                  {job.command && (
                    <div className="detail-item">
                      <label>{t('cron.command')}</label>
                      <span>{job.command}</span>
                    </div>
                  )}
                  <div className="detail-item">
                    <label>{t('cron.createdAt')}</label>
                    <span>{new Date(job.created_at).toLocaleString()}</span>
                  </div>
                  {job.last_run && (
                    <div className="detail-item">
                      <label>{t('cron.lastRun')}</label>
                      <span>{new Date(job.last_run).toLocaleString()}</span>
                    </div>
                  )}
                  {job.last_error && (
                    <div className="detail-item error">
                      <label>{t('cron.errorLabel')}</label>
                      <span>{job.last_error}</span>
                    </div>
                  )}
                </div>

                <div className="job-actions">
                  <button
                    className="action-btn secondary"
                    onClick={() => handleEdit(job)}
                    disabled={actionLoading}
                  >
                    {t('cron.edit')}
                  </button>
                  <button
                    className={`action-btn ${job.enabled ? 'warning' : 'success'}`}
                    onClick={() => handleTogglePause(job)}
                    disabled={actionLoading}
                  >
                    {job.enabled ? t('cron.pause') : t('cron.resume')}
                  </button>
                  <button
                    className="action-btn danger"
                    onClick={() => handleDelete(job)}
                    disabled={actionLoading}
                  >
                    {t('cron.delete')}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {showDialog && (
        <div className="dialog-overlay" onClick={() => setShowDialog(false)}>
          <div className="dialog" onClick={(e) => e.stopPropagation()}>
            <h2>{editingJob ? t('cron.editTask') : t('cron.addTaskDialog')}</h2>

            <div className="dialog-section">
              <div className="form-item">
                <label>{t('cron.taskName')}</label>
                <input
                  type="text"
                  value={formData.name || ''}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  placeholder={t('cron.taskNamePlaceholder')}
                />
              </div>

              <div className="form-item">
                <label>{t('cron.cronExprLabel')}</label>
                <input
                  type="text"
                  value={formData.cron || ''}
                  onChange={(e) => setFormData({ ...formData, cron: e.target.value })}
                  placeholder={t('cron.cronExprPlaceholder')}
                />
                <small>{t('cron.cronExprHint')}</small>
              </div>

              <div className="form-item">
                <label>{t('cron.messageLabel')}</label>
                <input
                  type="text"
                  value={formData.message || ''}
                  onChange={(e) => setFormData({ ...formData, message: e.target.value })}
                  placeholder={t('cron.messagePlaceholder')}
                />
                <small>{t('cron.messageHint')}</small>
              </div>

              <div className="form-item">
                <label>{t('cron.commandLabel')}</label>
                <input
                  type="text"
                  value={formData.command || ''}
                  onChange={(e) => setFormData({ ...formData, command: e.target.value })}
                  placeholder={t('cron.commandPlaceholder')}
                />
              </div>

              <div className="form-item">
                <label>
                  <input
                    type="checkbox"
                    checked={formData.enabled || false}
                    onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
                  />
                  {t('cron.enableTask')}
                </label>
              </div>
            </div>

            <div className="dialog-actions">
              <button
                className="action-btn secondary"
                onClick={() => setShowDialog(false)}
                disabled={actionLoading}
              >
                {t('cron.cancel')}
              </button>
              <button
                className="action-btn primary"
                onClick={handleSave}
                disabled={actionLoading}
              >
                {actionLoading ? t('cron.saving') : t('cron.save')}
              </button>
            </div>

            {actionMessage && <div className="action-message">{actionMessage}</div>}
          </div>
        </div>
      )}

      {actionLoading && (
        <div className="loading-overlay">
          <div className="loading-spinner"></div>
          <p>{actionMessage || t('cron.processing')}</p>
        </div>
      )}
    </div>
  );
}
