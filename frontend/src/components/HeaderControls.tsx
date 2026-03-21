import { useTheme } from '../hooks/useTheme';
import { useTranslation } from 'react-i18next';

export default function HeaderControls() {
  const { theme, toggle: toggleTheme } = useTheme();
  const { i18n } = useTranslation();

  const toggleLang = () => {
    const next = i18n.language === 'en' ? 'zh' : 'en';
    i18n.changeLanguage(next);
    localStorage.setItem('lang', next);
  };

  return (
    <div className="flex items-center gap-1">
      <button onClick={toggleTheme} className="p-1.5 rounded hover:bg-gray-700 text-sm" title="Toggle theme">
        {theme === 'dark' ? '☀️' : '🌙'}
      </button>
      <button onClick={toggleLang} className="p-1.5 rounded hover:bg-gray-700 text-sm" title="Switch language">
        {i18n.language === 'en' ? '中文' : 'EN'}
      </button>
    </div>
  );
}
