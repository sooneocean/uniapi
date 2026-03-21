import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import ProviderSettings from './ProviderSettings';
import UserSettings from './UserSettings';
import UsageDashboard from './UsageDashboard';
import AnalyticsDashboard from './AnalyticsDashboard';
import APIKeySettings from './APIKeySettings';
import AdminDashboard from './AdminDashboard';
import ModelAliases from './ModelAliases';
import KnowledgeBase from './KnowledgeBase';
import PluginManager from './PluginManager';
import PromptTemplates from './PromptTemplates';
import DataSettings from './DataSettings';
import WorkflowBuilder from './WorkflowBuilder';
import ThemeEditor from './ThemeEditor';

type Tab = 'dashboard' | 'providers' | 'users' | 'usage' | 'analytics' | 'apikeys' | 'aliases' | 'knowledge' | 'plugins' | 'templates' | 'workflows' | 'themes' | 'data';

interface SettingsProps {
  onClose: () => void;
  userRole?: string;
}

export default function Settings({ onClose, userRole }: SettingsProps) {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<Tab>(userRole === 'admin' ? 'dashboard' : 'providers');

  const tabs: { id: Tab; label: string; adminOnly?: boolean }[] = [
    { id: 'dashboard', label: 'Dashboard', adminOnly: true },
    { id: 'providers', label: t('settings.providers') },
    { id: 'users', label: t('settings.users'), adminOnly: true },
    { id: 'usage', label: t('settings.usage') },
    { id: 'analytics', label: 'Analytics' },
    { id: 'apikeys', label: t('settings.apiKeys') },
    { id: 'aliases', label: 'Model Aliases' },
    { id: 'knowledge', label: 'Knowledge' },
    { id: 'plugins', label: 'Plugins' },
    { id: 'templates', label: 'Templates' },
    { id: 'workflows', label: 'Workflows' },
    { id: 'themes', label: 'Themes' },
    { id: 'data', label: 'Data' },
  ];

  const visibleTabs = tabs.filter((t) => !t.adminOnly || userRole === 'admin');

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-gray-800 rounded-none md:rounded-xl shadow-2xl w-full md:max-w-2xl md:mx-4 h-full md:max-h-[85vh] flex flex-col" style={{ background: 'var(--bg-secondary)' }}>
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-700">
          <h1 className="text-white text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>{t('settings.title')}</h1>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-white transition-colors text-xl leading-none"
            aria-label="Close settings"
          >
            &times;
          </button>
        </div>

        {/* Tab bar — scrollable on mobile */}
        <div className="flex gap-1 px-6 pt-3 border-b border-gray-700 overflow-x-auto">
          {visibleTabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`px-4 py-2 text-sm font-medium rounded-t-lg transition-colors whitespace-nowrap flex-shrink-0 ${
                activeTab === tab.id
                  ? 'bg-gray-700 text-white border-b-2 border-blue-500'
                  : 'text-gray-400 hover:text-gray-200 hover:bg-gray-700/50'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Tab content */}
        <div className="flex-1 overflow-y-auto px-6 py-4">
          {activeTab === 'dashboard' && userRole === 'admin' && <AdminDashboard />}
          {activeTab === 'providers' && <ProviderSettings />}
          {activeTab === 'users' && userRole === 'admin' && <UserSettings />}
          {activeTab === 'usage' && <UsageDashboard />}
          {activeTab === 'analytics' && <AnalyticsDashboard />}
          {activeTab === 'apikeys' && <APIKeySettings />}
          {activeTab === 'aliases' && <ModelAliases />}
          {activeTab === 'knowledge' && <KnowledgeBase />}
          {activeTab === 'plugins' && <PluginManager />}
          {activeTab === 'templates' && <PromptTemplates />}
          {activeTab === 'workflows' && <WorkflowBuilder />}
          {activeTab === 'themes' && <ThemeEditor />}
          {activeTab === 'data' && <DataSettings />}
        </div>
      </div>
    </div>
  );
}
