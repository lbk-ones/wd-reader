import {Select} from 'antd';
import {useI18n} from './index.jsx';

export default function LanguageSelect({compact = false}) {
    const {language, setLanguage, t} = useI18n();
    return <label className={`language-setting${compact ? ' language-setting--compact' : ''}`}>
        {!compact && <span>{t('语言')}</span>}
        <Select size={compact ? 'small' : undefined} aria-label={t('界面语言')} value={language} onChange={setLanguage}
                options={[{value: 'zh-CN', label: '简体中文'}, {value: 'en', label: 'English'}]}/>
    </label>;
}
