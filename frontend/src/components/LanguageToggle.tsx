import { useTranslation } from 'react-i18next';

export default function LanguageToggle() {
  const { i18n } = useTranslation();
  const toggle = () => {
    const next = i18n.language === 'en' ? 'zh' : 'en';
    i18n.changeLanguage(next);
    localStorage.setItem('lang', next);
  };
  return (
    <button onClick={toggle} className="text-sm px-2 py-1 rounded hover:bg-gray-700" title="Switch language">
      {i18n.language === 'en' ? '中文' : 'EN'}
    </button>
  );
}
